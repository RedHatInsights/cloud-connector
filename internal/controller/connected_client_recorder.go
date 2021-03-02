package controller

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/queue"

	"github.com/google/uuid"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

type ConnectedClientRecorder interface {
	RecordConnectedClient(context.Context, domain.Identity, domain.AccountID, domain.ClientID, interface{}) error
}

func NewConnectedClientRecorder(impl string, cfg *config.Config) (ConnectedClientRecorder, error) {

	switch impl {
	case "inventory":
		kafkaProducerCfg := &queue.ProducerConfig{
			Brokers:    cfg.InventoryKafkaBrokers,
			Topic:      cfg.InventoryKafkaTopic,
			BatchSize:  cfg.InventoryKafkaBatchSize,
			BatchBytes: cfg.InventoryKafkaBatchBytes,
		}

		kafkaProducer := queue.StartProducer(kafkaProducerCfg)

		connectedClientRecorder := InventoryBasedConnectedClientRecorder{
			KafkaWriter:          kafkaProducer,
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

type inventoryMessageEnvelope struct {
	Operation        string      `json:"operation"`
	PlatformMetadata interface{} `json:"platform_metadata"`
	Data             interface{} `json:"data"`
}

type platformMetadata struct {
	RequestID string `json:"request_id"`
}

type InventoryBasedConnectedClientRecorder struct {
	KafkaWriter          *kafka.Writer
	StaleTimestampOffset time.Duration
	ReporterName         string
}

func (ibccr *InventoryBasedConnectedClientRecorder) RecordConnectedClient(ctx context.Context, identity domain.Identity, account domain.AccountID, clientID domain.ClientID, canonicalFacts interface{}) error {

	requestID, _ := uuid.NewUUID()

	logger := logger.Log.WithFields(logrus.Fields{"request_id": requestID.String(),
		"account":   account,
		"client_id": clientID})

	staleTimestamp := time.Now().Add(ibccr.StaleTimestampOffset)

	hostData := canonicalFacts.(map[string]interface{})

	hostData["account"] = string(account)
	hostData["rhc_client_id"] = clientID
	hostData["stale_timestamp"] = staleTimestamp.UTC().Format("2006-01-02T15:04:05Z07:00")
	hostData["reporter"] = ibccr.ReporterName

	metadata := platformMetadata{RequestID: requestID.String()}
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

	go func() {
		metrics.responseKafkaWriterGoRoutineGauge.Inc()
		defer metrics.responseKafkaWriterGoRoutineGauge.Dec()

		err = ibccr.KafkaWriter.WriteMessages(ctx,
			kafka.Message{
				Value: jsonInventoryMessage,
			})

		logger.Debug("Inventory kafka message written")

		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Error writing response message to kafka")

			if errors.Is(err, context.Canceled) != true {
				metrics.responseKafkaWriterFailureCounter.Inc()
			}
		} else {
			metrics.responseKafkaWriterSuccessCounter.Inc()
		}
	}()

	return nil
}

type FakeConnectedClientRecorder struct {
}

func (fccr *FakeConnectedClientRecorder) RecordConnectedClient(ctx context.Context, identity domain.Identity, account domain.AccountID, clientID domain.ClientID, canonicalFacts interface{}) error {
	logger := logger.Log.WithFields(logrus.Fields{"account": account, "client_id": clientID})

	logger.Debug("FAKE: connected client recorder: ", canonicalFacts)

	return nil
}
