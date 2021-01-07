package queue

type ProducerConfig struct {
	Brokers    []string
	Topic      string
	BatchSize  int
	BatchBytes int
}

type ConsumerConfig struct {
	Brokers        []string
	Topic          string
	GroupID        string
	ConsumerOffset int64
}
