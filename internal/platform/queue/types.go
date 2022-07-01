package queue

import (
	"fmt"
	"strings"
)

type ProducerConfig struct {
	Brokers    []string
	SaslConfig *SaslConfig
	Topic      string
	BatchSize  int
	BatchBytes int
	Balancer   string
}

func (this ProducerConfig) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Brokers: %s\n", this.Brokers)
	fmt.Fprintf(&b, "SaslConfig: %s\n", this.SaslConfig)
	fmt.Fprintf(&b, "Topic: %s\n", this.Topic)
	fmt.Fprintf(&b, "BatchSize: %d\n", this.BatchSize)
	fmt.Fprintf(&b, "BatchBytes: %d\n", this.BatchBytes)
	fmt.Fprintf(&b, "Balancer: %s\n", this.Balancer)
	return b.String()
}

type ConsumerConfig struct {
	Brokers        []string
	SaslConfig     *SaslConfig
	Topic          string
	GroupID        string
	ConsumerOffset int64
}

func (this ConsumerConfig) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Brokers: %s\n", this.Brokers)
	fmt.Fprintf(&b, "SaslConfig: %s\n", this.SaslConfig)
	fmt.Fprintf(&b, "Topic: %s\n", this.Topic)
	fmt.Fprintf(&b, "GroupID: %s\n", this.GroupID)
	fmt.Fprintf(&b, "ConsumerOffset: %d\n", this.ConsumerOffset)
	return b.String()
}

type SaslConfig struct {
	SaslMechanism        string
	SaslSecurityProtocol string
	SaslUsername         string
	SaslPassword         string
	KafkaCA              string
}

func (this SaslConfig) String() string {
	return fmt.Sprintf("SaslMechanism: %s\n", this.SaslMechanism)
}
