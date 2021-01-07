package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/RedHatInsights/cloud-connector/internal/platform/queue"

	kafka "github.com/segmentio/kafka-go"
)

type MessageDispatcherFactory struct {
	readerConfig *queue.ConsumerConfig
}

func NewMessageDispatcherFactory(cfg *queue.ConsumerConfig) *MessageDispatcherFactory {
	return &MessageDispatcherFactory{
		readerConfig: cfg,
	}
}

func (fact *MessageDispatcherFactory) NewDispatcher(account, nodeID string) *MessageDispatcher {
	log.Println("Creating a new work dispatcher")
	r := queue.StartConsumer(fact.readerConfig)
	return &MessageDispatcher{
		account: account,
		nodeID:  nodeID,
		reader:  r,
	}
}

type MessageDispatcher struct {
	account string
	nodeID  string
	reader  *kafka.Reader
}

func (md *MessageDispatcher) GetKey() string {
	return fmt.Sprintf("%s:%s", md.account, md.nodeID)
}

func (md *MessageDispatcher) StartDispatchingMessages(ctx context.Context, c chan<- Message) {
	defer func() {
		err := md.reader.Close()
		if err != nil {
			log.Println("Kafka job reader - error closing consumer: ", err)
			return
		}
		log.Println("Kafka job reader leaving...")
	}()

	for {
		log.Printf("Kafka job reader - waiting on a message from kafka...")
		m, err := md.reader.ReadMessage(ctx)
		if err != nil {
			// FIXME:  do we need to call cancel here??
			log.Println("Kafka job reader - error reading message: ", err)
			break
		}

		log.Printf("Kafka job reader - received message from %s-%d [%d]: %s: %s\n",
			m.Topic,
			m.Partition,
			m.Offset,
			string(m.Key),
			string(m.Value))

		if string(m.Key) == md.GetKey() {
			// FIXME:
			var w Message
			if err := json.Unmarshal(m.Value, &w); err != nil {
				log.Println("Unable to unmarshal message from kafka queue")
				continue
			}
			c <- w
		} else {
			log.Println("Kafka job reader - received message but did not send. Account number not found.")
		}
	}
}
