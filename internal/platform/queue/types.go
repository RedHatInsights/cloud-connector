package queue

type ProducerConfig struct {
	Brokers    []string
	SaslConfig *SaslConfig
	Topic      string
	BatchSize  int
	BatchBytes int
	Balancer   string
}

type ConsumerConfig struct {
	Brokers        []string
	SaslConfig     *SaslConfig
	Topic          string
	GroupID        string
	ConsumerOffset int64
}

type SaslConfig struct {
	SaslMechanism        string
	SaslSecurityProtocol string
	SaslUsername         string
	SaslPassword         string
	KafkaCA              string
}
