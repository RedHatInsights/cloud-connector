package controller

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/platform/queue"

	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type EventType string

type ConnectedClientEventAnnouncer interface {
	AnnounceEvent(context.Context, EventType, domain.RhcClient) error
}

func NewConnectedClientEventAnnouncer(impl string, cfg *config.Config) (ConnectedClientEventAnnouncer, error) {

	switch impl {
	case "kafka":
		kafkaProducerCfg := &queue.ProducerConfig{
			Brokers:    cfg.ConnectionEventsKafkaBrokers,
			Topic:      cfg.ConnectionEventsKafkaTopic,
			BatchSize:  cfg.ConnectionEventsKafkaBatchSize,
			BatchBytes: cfg.ConnectionEventsKafkaBatchBytes,
		}

		kafkaProducer := queue.StartProducer(kafkaProducerCfg)

		connectedClientEventAnnouncer := KafkaBasedConnectedClientEventAnnouncer{
			kafkaWriter: kafkaProducer,

			kafkaWriterGoRoutineGauge: promauto.NewGauge(prometheus.GaugeOpts{
				Name: "cloud_connector_event_kafka_writer_go_routine_count",
				Help: "The total number of active kakfa event writer go routines",
			}),

			kafkaWriterSuccessCounter: promauto.NewCounter(prometheus.CounterOpts{
				Name: "cloud_connector_event_kakfa_writer_success_count",
				Help: "The number of events were sent to the kafka topic",
			}),

			kafkaWriterFailureCounter: promauto.NewCounter(prometheus.CounterOpts{
				Name: "cloud_connector_event_kafka_writer_failure_count",
				Help: "The number of events that failed to get produced to kafka topic",
			}),
		}

		return &connectedClientEventAnnouncer, nil
	case "fake":
		return &FakeConnectedClientEventAnnouncer{}, nil
	default:
		return nil, errors.New("Invalid ConnectedClientEventAnnouncer impl requested")
	}
}

type KafkaBasedConnectedClientEventAnnouncer struct {
	kafkaWriter               *kafka.Writer
	kafkaWriterGoRoutineGauge prometheus.Gauge
	kafkaWriterSuccessCounter prometheus.Counter
	kafkaWriterFailureCounter prometheus.Counter
}

func (kbccea *KafkaBasedConnectedClientEventAnnouncer) AnnounceEvent(ctx context.Context, eventType EventType, rhcClient domain.RhcClient) error {

	account := rhcClient.Account
	clientID := rhcClient.ClientID

	logger := logger.Log.WithFields(logrus.Fields{"account": account, "client_id": clientID})

	envelope := struct{}{}

	jsonMessage, err := json.Marshal(envelope)
	if err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("JSON marshal of connection event message failed")
		return err
	}

	headers := []kafka.Header{
		{Key: "rhc_client_id", Value: []byte(clientID)},
		{Key: "type", Value: []byte(eventType)},
	}

	go func() {
		kbccea.kafkaWriterGoRoutineGauge.Inc()
		defer kbccea.kafkaWriterGoRoutineGauge.Dec()

		err = kbccea.kafkaWriter.WriteMessages(ctx,
			kafka.Message{
				Value:   jsonMessage,
				Headers: headers,
			})

		logger.Debug("Connected client event kafka message written")

		if err != nil {
			logger.WithFields(logrus.Fields{"error": err}).Error("Error writing response message to kafka")

			if errors.Is(err, context.Canceled) != true {
				kbccea.kafkaWriterFailureCounter.Inc()
			}
		} else {
			kbccea.kafkaWriterSuccessCounter.Inc()
		}
	}()

	return nil
}

type FakeConnectedClientEventAnnouncer struct {
}

func (fccr *FakeConnectedClientEventAnnouncer) AnnounceEvent(ctx context.Context, eventType EventType, rhcClient domain.RhcClient) error {
	logger := logger.Log.WithFields(logrus.Fields{"account": rhcClient.Account, "client_id": rhcClient.ClientID})

	logger.Debug("FAKE: connected client event type: ", eventType, " - ", rhcClient.CanonicalFacts)

	return nil
}
