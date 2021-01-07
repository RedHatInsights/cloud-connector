package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	ENV_PREFIX = "RECEPTOR_CONTROLLER"

	HANDSHAKE_READ_WAIT                                = "WebSocket_Handshake_Read_Wait"
	WRITE_WAIT                                         = "WebSocket_Write_Wait"
	PONG_WAIT                                          = "WebSocket_Pong_Wait"
	PING_PERIOD                                        = "WebSocket_Ping_Period"
	RECEPTOR_SYNC_PING_TIMEOUT                         = "Receptor_Sync_Ping_Timeout"
	HTTP_SHUTDOWN_TIMEOUT                              = "HTTP_Shutdown_Timeout"
	MAX_MESSAGE_SIZE                                   = "WebSocket_Max_Message_Size"
	SOCKET_BUFFER_SIZE                                 = "WebSocket_IO_Buffer_Size"
	BUFFERED_CHANNEL_SIZE                              = "WebSocket_Buffered_Channel_Size"
	SERVICE_TO_SERVICE_CREDENTIALS                     = "Service_To_Service_Credentials"
	PROFILE                                            = "Enable_Profile"
	BROKERS                                            = "Kafka_Brokers"
	JOBS_TOPIC                                         = "Kafka_Jobs_Topic"
	JOBS_GROUP_ID                                      = "Kafka_Jobs_Group_Id"
	JOBS_CONSUMER_OFFSET                               = "Kafka_Jobs_Consumer_Offset"
	RESPONSES_TOPIC                                    = "Kafka_Responses_Topic"
	RESPONSES_BATCH_SIZE                               = "Kafka_Responses_Batch_Size"
	RESPONSES_BATCH_BYTES                              = "Kafka_Responses_Batch_Bytes"
	DEFAULT_BROKER_ADDRESS                             = "kafka:29092"
	REDIS_HOST                                         = "Redis_Host"
	REDIS_PORT                                         = "Redis_Port"
	REDIS_PASSWORD                                     = "Redis_Password"
	REDIS_DB                                           = "Redis_DB"
	JOB_RECEIVER_RECEPTOR_PROXY_CLIENT_ID              = "Job_Receiver_Receptor_Proxy_ClientID"
	JOB_RECEIVER_RECEPTOR_PROXY_PSK                    = "Job_Receiver_Receptor_Proxy_PSK"
	JOB_RECEIVER_RECEPTOR_PROXY_SCHEME                 = "Job_Receiver_Receptor_Proxy_Scheme"
	JOB_RECEIVER_RECEPTOR_PROXY_PORT                   = "Job_Receiver_Receptor_Proxy_Port"
	JOB_RECEIVER_RECEPTOR_PROXY_TIMEOUT                = "Job_Receiver_Receptor_Proxy_Timeout"
	GATEWAY_CONNECTION_REGISTRAR_IMPL                  = "Gateway_Connection_Registrar_Impl"
	GATEWAY_ACTIVE_CONNECTION_REGISTRAR_POLL_MIN_DELAY = "Gateway_Active_Connection_Registrar_Poll_Min_Delay"
	GATEWAY_ACTIVE_CONNECTION_REGISTRAR_POLL_MAX_DELAY = "Gateway_Active_Connection_Registrar_Poll_Max_Delay"
	GATEWAY_CLUSTER_SERVICE_NAME                       = "Gateway_Cluster_Service_Name"
	NODE_ID                                            = "ReceptorControllerNodeId"
	PROMETHEUS_PUSH_GATEWAY                            = "Prometheus_Push_Gateway"
)

type Config struct {
	HandshakeReadWait                            time.Duration
	WriteWait                                    time.Duration
	PongWait                                     time.Duration
	PingPeriod                                   time.Duration
	ReceptorSyncPingTimeout                      time.Duration
	HttpShutdownTimeout                          time.Duration
	MaxMessageSize                               int64
	SocketBufferSize                             int
	BufferedChannelSize                          int
	ServiceToServiceCredentials                  map[string]interface{}
	Profile                                      bool
	ReceptorControllerNodeId                     string
	KafkaBrokers                                 []string
	KafkaJobsTopic                               string
	KafkaResponsesTopic                          string
	KafkaResponsesBatchSize                      int
	KafkaResponsesBatchBytes                     int
	KafkaGroupID                                 string
	KafkaConsumerOffset                          int64
	RedisHost                                    string
	RedisPort                                    string
	RedisPassword                                string
	RedisDB                                      int
	JobReceiverReceptorProxyClientID             string
	JobReceiverReceptorProxyPSK                  string
	JobReceiverReceptorProxyScheme               string
	JobReceiverReceptorProxyPort                 int
	JobReceiverReceptorProxyTimeout              time.Duration
	GatewayConnectionRegistrarImpl               string
	GatewayActiveConnectionRegistrarPollMinDelay int
	GatewayActiveConnectionRegistrarPollMaxDelay int
	GatewayClusterServiceName                    string
	PrometheusPushGateway                        string
}

func (c Config) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s\n", HANDSHAKE_READ_WAIT, c.HandshakeReadWait)
	fmt.Fprintf(&b, "%s: %s\n", WRITE_WAIT, c.WriteWait)
	fmt.Fprintf(&b, "%s: %s\n", PONG_WAIT, c.PongWait)
	fmt.Fprintf(&b, "%s: %s\n", PING_PERIOD, c.PingPeriod)
	fmt.Fprintf(&b, "%s: %s\n", RECEPTOR_SYNC_PING_TIMEOUT, c.ReceptorSyncPingTimeout)
	fmt.Fprintf(&b, "%s: %s\n", HTTP_SHUTDOWN_TIMEOUT, c.HttpShutdownTimeout)
	fmt.Fprintf(&b, "%s: %d\n", MAX_MESSAGE_SIZE, c.MaxMessageSize)
	fmt.Fprintf(&b, "%s: %d\n", SOCKET_BUFFER_SIZE, c.SocketBufferSize)
	fmt.Fprintf(&b, "%s: %d\n", BUFFERED_CHANNEL_SIZE, c.BufferedChannelSize)
	fmt.Fprintf(&b, "%s: %t\n", PROFILE, c.Profile)
	fmt.Fprintf(&b, "%s: %s\n", NODE_ID, c.ReceptorControllerNodeId)
	fmt.Fprintf(&b, "%s: %s\n", BROKERS, c.KafkaBrokers)
	fmt.Fprintf(&b, "%s: %s\n", JOBS_TOPIC, c.KafkaJobsTopic)
	fmt.Fprintf(&b, "%s: %s\n", RESPONSES_TOPIC, c.KafkaResponsesTopic)
	fmt.Fprintf(&b, "%s: %d\n", RESPONSES_BATCH_SIZE, c.KafkaResponsesBatchSize)
	fmt.Fprintf(&b, "%s: %d\n", RESPONSES_BATCH_BYTES, c.KafkaResponsesBatchBytes)
	fmt.Fprintf(&b, "%s: %s\n", JOBS_GROUP_ID, c.KafkaGroupID)
	fmt.Fprintf(&b, "%s: %d\n", JOBS_CONSUMER_OFFSET, c.KafkaConsumerOffset)
	fmt.Fprintf(&b, "%s: %s\n", REDIS_HOST, c.RedisHost)
	fmt.Fprintf(&b, "%s: %s\n", REDIS_PORT, c.RedisPort)
	fmt.Fprintf(&b, "%s: %d\n", REDIS_DB, c.RedisDB)
	fmt.Fprintf(&b, "%s: %s\n", JOB_RECEIVER_RECEPTOR_PROXY_CLIENT_ID, c.JobReceiverReceptorProxyClientID)
	fmt.Fprintf(&b, "%s: %s\n", JOB_RECEIVER_RECEPTOR_PROXY_SCHEME, c.JobReceiverReceptorProxyScheme)
	fmt.Fprintf(&b, "%s: %d\n", JOB_RECEIVER_RECEPTOR_PROXY_PORT, c.JobReceiverReceptorProxyPort)
	fmt.Fprintf(&b, "%s: %s\n", JOB_RECEIVER_RECEPTOR_PROXY_TIMEOUT, c.JobReceiverReceptorProxyTimeout)
	fmt.Fprintf(&b, "%s: %s\n", GATEWAY_CONNECTION_REGISTRAR_IMPL, c.GatewayConnectionRegistrarImpl)
	fmt.Fprintf(&b, "%s: %d\n", GATEWAY_ACTIVE_CONNECTION_REGISTRAR_POLL_MIN_DELAY, c.GatewayActiveConnectionRegistrarPollMinDelay)
	fmt.Fprintf(&b, "%s: %d\n", GATEWAY_ACTIVE_CONNECTION_REGISTRAR_POLL_MAX_DELAY, c.GatewayActiveConnectionRegistrarPollMaxDelay)
	fmt.Fprintf(&b, "%s: %s\n", GATEWAY_CLUSTER_SERVICE_NAME, c.GatewayClusterServiceName)
	fmt.Fprintf(&b, "%s: %s\n", PROMETHEUS_PUSH_GATEWAY, c.PrometheusPushGateway)
	return b.String()
}

func GetConfig() *Config {
	options := viper.New()

	options.SetDefault(HANDSHAKE_READ_WAIT, 5)
	options.SetDefault(WRITE_WAIT, 5)
	options.SetDefault(PONG_WAIT, 25)
	options.SetDefault(RECEPTOR_SYNC_PING_TIMEOUT, 10)
	options.SetDefault(HTTP_SHUTDOWN_TIMEOUT, 2)
	options.SetDefault(MAX_MESSAGE_SIZE, 1*1024*1024)
	options.SetDefault(SOCKET_BUFFER_SIZE, 1024)
	options.SetDefault(BUFFERED_CHANNEL_SIZE, 10)
	options.SetDefault(SERVICE_TO_SERVICE_CREDENTIALS, "")
	options.SetDefault(PROFILE, false)
	options.SetDefault(NODE_ID, "node-cloud-receptor-controller")
	options.SetDefault(BROKERS, []string{DEFAULT_BROKER_ADDRESS})
	options.SetDefault(JOBS_TOPIC, "platform.receptor-controller.jobs")
	options.SetDefault(RESPONSES_TOPIC, "platform.receptor-controller.responses")
	options.SetDefault(RESPONSES_BATCH_SIZE, 100)
	options.SetDefault(RESPONSES_BATCH_BYTES, 1048576)
	options.SetDefault(JOBS_GROUP_ID, "receptor-controller")
	options.SetDefault(JOBS_CONSUMER_OFFSET, -1)
	options.SetDefault(REDIS_HOST, "localhost")
	options.SetDefault(REDIS_PORT, "6379")
	options.SetDefault(REDIS_PASSWORD, "")
	options.SetDefault(REDIS_DB, 0)
	options.SetDefault(JOB_RECEIVER_RECEPTOR_PROXY_CLIENT_ID, "job_receiver")
	options.SetDefault(JOB_RECEIVER_RECEPTOR_PROXY_PSK, "")
	options.SetDefault(JOB_RECEIVER_RECEPTOR_PROXY_SCHEME, "http")
	options.SetDefault(JOB_RECEIVER_RECEPTOR_PROXY_PORT, 9090)
	options.SetDefault(JOB_RECEIVER_RECEPTOR_PROXY_TIMEOUT, 10)
	options.SetDefault(GATEWAY_CONNECTION_REGISTRAR_IMPL, "local")
	options.SetDefault(GATEWAY_ACTIVE_CONNECTION_REGISTRAR_POLL_MIN_DELAY, 5*1000)
	options.SetDefault(GATEWAY_ACTIVE_CONNECTION_REGISTRAR_POLL_MAX_DELAY, 10*1000)
	options.SetDefault(GATEWAY_CLUSTER_SERVICE_NAME, "receptor-gateway-internal")
	options.SetDefault(PROMETHEUS_PUSH_GATEWAY, "prometheus-push.insights-push-prod.svc.cluster.local:9091")
	options.SetEnvPrefix(ENV_PREFIX)
	options.AutomaticEnv()

	writeWait := options.GetDuration(WRITE_WAIT) * time.Second
	pongWait := options.GetDuration(PONG_WAIT) * time.Second
	pingPeriod := calculatePingPeriod(pongWait)

	return &Config{
		HandshakeReadWait:                options.GetDuration(HANDSHAKE_READ_WAIT) * time.Second,
		WriteWait:                        writeWait,
		PongWait:                         pongWait,
		PingPeriod:                       pingPeriod,
		ReceptorSyncPingTimeout:          options.GetDuration(RECEPTOR_SYNC_PING_TIMEOUT) * time.Second,
		HttpShutdownTimeout:              options.GetDuration(HTTP_SHUTDOWN_TIMEOUT) * time.Second,
		MaxMessageSize:                   options.GetInt64(MAX_MESSAGE_SIZE),
		SocketBufferSize:                 options.GetInt(SOCKET_BUFFER_SIZE),
		BufferedChannelSize:              options.GetInt(BUFFERED_CHANNEL_SIZE),
		ServiceToServiceCredentials:      options.GetStringMap(SERVICE_TO_SERVICE_CREDENTIALS),
		Profile:                          options.GetBool(PROFILE),
		ReceptorControllerNodeId:         options.GetString(NODE_ID),
		KafkaBrokers:                     options.GetStringSlice(BROKERS),
		KafkaJobsTopic:                   options.GetString(JOBS_TOPIC),
		KafkaResponsesTopic:              options.GetString(RESPONSES_TOPIC),
		KafkaResponsesBatchSize:          options.GetInt(RESPONSES_BATCH_SIZE),
		KafkaResponsesBatchBytes:         options.GetInt(RESPONSES_BATCH_BYTES),
		KafkaGroupID:                     options.GetString(JOBS_GROUP_ID),
		KafkaConsumerOffset:              options.GetInt64(JOBS_CONSUMER_OFFSET),
		RedisHost:                        options.GetString(REDIS_HOST),
		RedisPort:                        options.GetString(REDIS_PORT),
		RedisPassword:                    options.GetString(REDIS_PASSWORD),
		RedisDB:                          options.GetInt(REDIS_DB),
		JobReceiverReceptorProxyClientID: options.GetString(JOB_RECEIVER_RECEPTOR_PROXY_CLIENT_ID),
		JobReceiverReceptorProxyPSK:      options.GetString(JOB_RECEIVER_RECEPTOR_PROXY_PSK),
		JobReceiverReceptorProxyScheme:   options.GetString(JOB_RECEIVER_RECEPTOR_PROXY_SCHEME),
		JobReceiverReceptorProxyPort:     options.GetInt(JOB_RECEIVER_RECEPTOR_PROXY_PORT),
		JobReceiverReceptorProxyTimeout:  options.GetDuration(JOB_RECEIVER_RECEPTOR_PROXY_TIMEOUT) * time.Second,
		GatewayConnectionRegistrarImpl:   options.GetString(GATEWAY_CONNECTION_REGISTRAR_IMPL),
		GatewayActiveConnectionRegistrarPollMinDelay: options.GetInt(GATEWAY_ACTIVE_CONNECTION_REGISTRAR_POLL_MIN_DELAY),
		GatewayActiveConnectionRegistrarPollMaxDelay: options.GetInt(GATEWAY_ACTIVE_CONNECTION_REGISTRAR_POLL_MAX_DELAY),
		GatewayClusterServiceName:                    options.GetString(GATEWAY_CLUSTER_SERVICE_NAME),
		PrometheusPushGateway:                        options.GetString(PROMETHEUS_PUSH_GATEWAY),
	}
}

func calculatePingPeriod(pongWait time.Duration) time.Duration {
	pingPeriod := (pongWait * 9) / 10
	return pingPeriod
}
