package controller

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/queue"
	"github.com/RedHatInsights/cloud-connector/internal/platform/utils/identity_utils"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

const (
	playbookWorkerDispatcherKey = "rhc-worker-playbook"
	packageManagerDispatcherKey = "package-manager"
	inventoryTagNamespace       = "rhc_client"
)

type ConnectedClientRecorder interface {
	RecordConnectedClient(context.Context, domain.Identity, domain.ConnectorClientState) error
}

func NewConnectedClientRecorder(impl string, cfg *config.Config) (ConnectedClientRecorder, error) {

	var kafkaProducerCfg *kafka.ConfigMap

	switch impl {
	case "inventory":
		if cfg.KafkaSASLMechanism != "" {
			kafkaProducerCfg = &kafka.ConfigMap{
				"bootstrap.servers":  strings.Join(cfg.InventoryKafkaBrokers, ","),
				"security.protocol":  cfg.KafkaProtocol,
				"sasl.mechanism":     cfg.KafkaSASLMechanism,
				"ssl.ca.location":    cfg.KafkaCA,
				"sasl.username":      cfg.KafkaUsername,
				"sasl.password":      cfg.KafkaPassword,
				"batch.num.messages": cfg.InventoryKafkaBatchSize,
				"batch.size":         cfg.InventoryKafkaBatchSize,
			}
		} else {
			kafkaProducerCfg = &kafka.ConfigMap{
				"bootstrap.servers":  strings.Join(cfg.InventoryKafkaBrokers, ","),
				"batch.num.messages": cfg.InventoryKafkaBatchSize,
				"batch.size":         cfg.InventoryKafkaBatchSize,
			}
		}

		kafkaProducer := queue.StartProducer(kafkaProducerCfg)

		connectedClientRecorder := InventoryBasedConnectedClientRecorder{
			MessageProducer:      BuildInventoryMessageProducer(kafkaProducer, cfg.InventoryKafkaTopic),
			StaleTimestampOffset: cfg.InventoryStaleTimestampOffset,
			ReporterName:         cfg.InventoryReporterName,
		}

		return &connectedClientRecorder, nil
	case "fake":
		return &FakeConnectedClientRecorder{}, nil
	default:
		return nil, errors.New("Invalid ConnectedClientRecorder impl requested")
	}
}

func NewInventoryBasedConnectedClientRecorder(kafkaWriter InventoryMessageProducer, staleTimestampOffset time.Duration, reporterName string) (ConnectedClientRecorder, error) {
	connectedClientRecorder := InventoryBasedConnectedClientRecorder{
		MessageProducer:      kafkaWriter,
		StaleTimestampOffset: staleTimestampOffset,
		ReporterName:         reporterName,
	}

	return &connectedClientRecorder, nil
}

type inventoryMessageEnvelope struct {
	Operation        string      `json:"operation"`
	PlatformMetadata interface{} `json:"platform_metadata"`
	Data             interface{} `json:"data"`
}

type platformMetadata struct {
	RequestID   string `json:"request_id"`
	B64Identity string `json:"b64_identity"`
}

type InventoryBasedConnectedClientRecorder struct {
	MessageProducer      InventoryMessageProducer
	StaleTimestampOffset time.Duration
	ReporterName         string
}

type InventoryMessageProducer func(ctx context.Context, log *logrus.Entry, msg []byte) error

func BuildInventoryMessageProducer(kafkaProducer *kafka.Producer, topic string) InventoryMessageProducer {
	return func(ctx context.Context, log *logrus.Entry, msg []byte) error {

		err := kafkaProducer.Produce(
			&kafka.Message{
				TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
				Value:          msg,
			}, nil)

		log.Debug("Inventory kafka message written")

		if err != nil {
			log.WithFields(logrus.Fields{"error": err}).Error("Error writing response message to kafka")

			if errors.Is(err, context.Canceled) != true {
				metrics.inventoryKafkaWriterFailureCounter.Inc()
			}
		} else {
			metrics.inventoryKafkaWriterSuccessCounter.Inc()
		}

		return nil
	}
}

func (ibccr *InventoryBasedConnectedClientRecorder) RecordConnectedClient(ctx context.Context, identity domain.Identity, rhcClient domain.ConnectorClientState) error {

	logger := logger.Log.WithFields(logrus.Fields{
		"account":   rhcClient.Account,
		"org_id":    rhcClient.OrgID,
		"client_id": rhcClient.ClientID})

	if shouldHostBeRegisteredWithInventory(rhcClient) == false {
		logger.Debug("Skipping inventory registration")
		return nil
	}

	staleTimestamp := time.Now().Add(ibccr.StaleTimestampOffset)

	originalHostData := rhcClient.CanonicalFacts.(map[string]interface{})

	hostData := cleanupCanonicalFacts(logger, originalHostData)

	hostData["account"] = string(rhcClient.Account)
	hostData["org_id"] = string(rhcClient.OrgID)
	hostData["stale_timestamp"] = staleTimestamp.UTC().Format("2006-01-02T15:04:05Z07:00")
	hostData["reporter"] = ibccr.ReporterName

	var systemProfile = map[string]string{"rhc_client_id": string(rhcClient.ClientID)}
	hostData["system_profile"] = systemProfile

	certAuth, _ := identity_utils.AuthenticatedWithCertificate(identity)
	if certAuth == true {
		logger.Debug("Adding the owner_id to the inventory message")
		systemProfile["owner_id"] = string(rhcClient.ClientID)
	}

	tags := convertRHCTagsToInventoryTags(rhcClient.Tags)
	if tags != nil {
		hostData["tags"] = convertRHCTagsToInventoryTags(rhcClient.Tags)
	}

	requestID, _ := uuid.NewUUID()

	metadata := platformMetadata{RequestID: requestID.String(), B64Identity: string(identity)}

	envelope := inventoryMessageEnvelope{
		Operation:        "add_host",
		PlatformMetadata: metadata,
		Data:             hostData,
	}

	jsonInventoryMessage, err := json.Marshal(envelope)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("JSON marshal of inventory message failed")
		return err
	}

	logger = logger.WithFields(logrus.Fields{"request_id": requestID.String()})

	ibccr.MessageProducer(ctx, logger, jsonInventoryMessage)

	return nil
}

func shouldHostBeRegisteredWithInventory(connectorClient domain.ConnectorClientState) bool {
	return doesHostHaveCanonicalFacts(connectorClient) && (doesHostHaveDispatcher(connectorClient, playbookWorkerDispatcherKey) || doesHostHaveDispatcher(connectorClient, packageManagerDispatcherKey))
}

func doesHostHaveCanonicalFacts(connectorClient domain.ConnectorClientState) bool {
	if connectorClient.CanonicalFacts == nil {
		return false
	}

	canonicalFactMap := connectorClient.CanonicalFacts.(map[string]interface{})

	return len(canonicalFactMap) > 0
}

func doesHostHaveDispatcher(connectorClient domain.ConnectorClientState, dispatcherName string) bool {

	if connectorClient.Dispatchers == nil {
		return false
	}

	dispatchersMap := connectorClient.Dispatchers.(map[string]interface{})
	_, foundDispatcher := dispatchersMap[dispatcherName]
	return foundDispatcher
}

func cleanupCanonicalFacts(logger *logrus.Entry, canonicalFacts map[string]interface{}) map[string]interface{} {
	hostData := make(map[string]interface{})

	for key, value := range canonicalFacts {
		if value != nil {
			v := reflect.ValueOf(value)
			switch v.Kind() {
			case reflect.Array, reflect.Slice:
				// Do not pass an empty array to inventory
				if v.Len() > 0 {
					hostData[key] = value
				}
			case reflect.String:
				// Do not pass an empty string to inventory
				if len(v.String()) > 0 {
					hostData[key] = value
				}
			default:
				logger.Debugf("Unknown type in canonical facts map - key: %s, value: %s", key, value)
			}
		}
	}

	return hostData
}

func convertRHCTagsToInventoryTags(rhcTags domain.Tags) map[string]map[string][]string {
	tagData := make(map[string]map[string][]string)

	if rhcTags == nil {
		return nil
	}

	tags, ok := rhcTags.(map[string]interface{})
	if !ok {
		return nil
	}

	if len(tags) == 0 {
		return tagData
	}

	tagData[inventoryTagNamespace] = make(map[string][]string)

	for k, v := range tags {
		tagData[inventoryTagNamespace][k] = []string{v.(string)}
	}

	return tagData
}

type FakeConnectedClientRecorder struct {
}

func (fccr *FakeConnectedClientRecorder) RecordConnectedClient(ctx context.Context, identity domain.Identity, rhcClient domain.ConnectorClientState) error {
	logger := logger.Log.WithFields(logrus.Fields{"account": rhcClient.Account, "client_id": rhcClient.ClientID, "org_id": rhcClient.OrgID})

	logger.Debug("FAKE: connected client recorder: ", rhcClient.CanonicalFacts)

	return nil
}
