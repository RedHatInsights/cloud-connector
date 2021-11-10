package config

import (
	"fmt"
	"strings"
	"time"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/spf13/viper"
)

const (
	ENV_PREFIX = "CLOUD_CONNECTOR"

	URL_APP_NAME                                 = "URL_App_Name"
	URL_PATH_PREFIX                              = "URL_Path_Prefix"
	URL_BASE_PATH                                = "URL_Base_Path"
	OPENAPI_SPEC_FILE_PATH                       = "OpenAPI_Spec_File_Path"
	HTTP_SHUTDOWN_TIMEOUT                        = "HTTP_Shutdown_Timeout"
	SERVICE_TO_SERVICE_CREDENTIALS               = "Service_To_Service_Credentials"
	PROFILE                                      = "Enable_Profile"
	MQTT_BROKER_ADDRESS                          = "MQTT_Broker_Address"
	MQTT_BROKER_ADDRESS_DEFAULT                  = "ssl://localhost:8883"
	MQTT_CLIENT_ID                               = "MQTT_Client_Id"
	MQTT_USE_HOSTNAME_AS_CLIENT_ID               = "MQTT_Use_Hostname_As_Client_Id"
	MQTT_CLEAN_SESSION                           = "MQTT_Clean_Session"
	MQTT_RESUME_SUBS                             = "MQTT_Resume_Subs"
	MQTT_BROKER_TLS_CERT_FILE                    = "MQTT_Broker_Tls_Cert_File"
	MQTT_BROKER_TLS_KEY_FILE                     = "MQTT_Broker_Tls_Key_File"
	MQTT_BROKER_TLS_CA_CERT_FILE                 = "MQTT_Broker_Tls_CA_Cert_File"
	MQTT_BROKER_TLS_SKIP_VERIFY                  = "MQTT_Broker_Tls_Skip_Verify"
	MQTT_BROKER_JWT_GENERATOR_IMPL               = "MQTT_Broker_JWT_Generator_Impl"
	MQTT_BROKER_JWT_FILE                         = "MQTT_Broker_JWT_File"
	MQTT_TOPIC_PREFIX                            = "MQTT_Topic_Prefix"
	MQTT_CONTROL_SUBSCRIPTION_QOS                = "MQTT_Control_Subscription_QoS"
	MQTT_CONTROL_PUBLISH_QOS                     = "MQTT_Control_Publish_QoS"
	MQTT_DATA_SUBSCRIPTION_QOS                   = "MQTT_Data_Subscription_QoS"
	MQTT_DATA_PUBLISH_QOS                        = "MQTT_Data_Publish_QoS"
	MQTT_DISCONNECT_QUIESCE_TIME                 = "MQTT_Disconnect_Quiesce_Time"
	MQTT_MESSAGE_DISPATCHER_CONCURRENCY_LIMIT    = "MQTT_Message_Dispatcher_Concurrency_Limit"
	INVALID_HANDSHAKE_RECONNECT_DELAY            = "Invalid_Handshake_Reconnect_Delay"
	CLIENT_ID_TO_ACCOUNT_ID_IMPL                 = "Client_Id_To_Account_Id_Impl"
	CLIENT_ID_TO_ACCOUNT_ID_CONFIG_FILE          = "Client_Id_To_Account_Id_Config_File"
	CLIENT_ID_TO_ACCOUNT_ID_DEFAULT_ACCOUNT_ID   = "Client_Id_To_Account_Id_Default_Account_Id"
	CONNECTION_DATABASE_IMPL                     = "Connection_Database_Impl"
	CONNECTION_DATABASE_HOST                     = "Connection_Database_Host"
	CONNECTION_DATABASE_PORT                     = "Connection_Database_Port"
	CONNECTION_DATABASE_USER                     = "Connection_Database_User"
	CONNECTION_DATABASE_PASSWORD                 = "Connection_Database_Password"
	CONNECTION_DATABASE_NAME                     = "Connection_Database_Name"
	CONNECTION_DATABASE_SSL_MODE                 = "Connection_Database_SSL_Mode"
	CONNECTION_DATABASE_SSL_ROOT_CERT            = "Connection_Database_SSL_Root_Cert"
	CONNECTION_DATABASE_SQLITE_FILE              = "Connection_Database_Sqlite_File"
	CONNECTION_DATABASE_QUERY_TIMEOUT            = "Connection_Database_Query_Timeout"
	AUTH_GATEWAY_URL                             = "Auth_Gateway_Url"
	AUTH_GATEWAY_HTTP_CLIENT_TIMEOUT             = "Auth_Gateway_HTTP_Client_Timeout"
	DEFAULT_KAFKA_BROKER_ADDRESS                 = "kafka:29092"
	CONNECTED_CLIENT_RECORDER_IMPL               = "Connected_Client_Recorder_Impl"
	INVENTORY_KAFKA_BROKERS                      = "Inventory_Kafka_Brokers"
	INVENTORY_KAFKA_TOPIC                        = "Inventory_Kafka_Topic"
	INVENTORY_KAFKA_BATCH_SIZE                   = "Inventory_Kafka_Batch_Size"
	INVENTORY_KAFKA_BATCH_BYTES                  = "Inventory_Kafka_Batch_Bytes"
	INVENTORY_STALE_TIMESTAMP_OFFSET             = "Inventory_Stale_Timestamp_Offset"
	INVENTORY_STALE_TIMESTAMP_UPDATER_CHUNK_SIZE = "Inventory_Stale_Timestamp_Updater_Chunk_Size"
	INVENTORY_REPORTER_NAME                      = "Inventory_Reporter_Name"
	SOURCES_RECORDER_IMPL                        = "Sources_Recorder_Impl"
	SOURCES_BASE_URL                             = "Sources_Base_Url"
	SOURCES_HTTP_CLIENT_TIMEOUT                  = "Sources_HTTP_Client_Timeout"
	JWT_TOKEN_EXPIRY                             = "JWT_Token_Expiry_Minutes"
	JWT_PRIVATE_KEY_FILE                         = "JWT_Private_Key_File"
	JWT_PUBLIC_KEY_FILE                          = "JWT_Public_Key_File"
	RHC_MESSAGE_KAFKA_BROKERS                    = "RHC_Message_Kafka_Brokers"
	RHC_MESSAGE_KAFKA_TOPIC                      = "RHC_Message_Kafka_Topic"
	RHC_MESSAGE_KAFKA_TOPIC_DEFAULT              = "platform.cloud-connector.rhc-message-ingress"
	RHC_MESSAGE_KAFKA_BATCH_SIZE                 = "RHC_Message_Kafka_Batch_Size"
	RHC_MESSAGE_KAFKA_BATCH_BYTES                = "RHC_Message_Kafka_Batch_Bytes"
	RHC_MESSAGE_KAFKA_CONSUMER_GROUP             = "RHC_Message_Kafka_Consumer_Group"
	PENDO_API_ENDPOINT                           = "Pendo_Api_Endpoint"
	PENDO_REQUEST_TIMEOUT                        = "Pendo_Request_Timeout"
	PENDO_INTEGRATION_KEY                        = "Pendo_Integration_Key"
	PENDO_REQUEST_SIZE                           = "Pendo_Request_Size"
)

type Config struct {
	UrlAppName                              string
	UrlPathPrefix                           string
	UrlBasePath                             string
	OpenApiSpecFilePath                     string
	HttpShutdownTimeout                     time.Duration
	ServiceToServiceCredentials             map[string]interface{}
	Profile                                 bool
	MqttBrokerAddress                       string
	MqttClientId                            string
	MqttUseHostnameAsClientId               bool
	MqttCleanSession                        bool
	MqttResumeSubs                          bool
	MqttBrokerTlsCertFile                   string
	MqttBrokerTlsKeyFile                    string
	MqttBrokerTlsCACertFile                 string
	MqttBrokerTlsSkipVerify                 bool
	MqttBrokerJwtGeneratorImpl              string
	MqttBrokerJwtFile                       string
	MqttTopicPrefix                         string
	MqttControlSubscriptionQoS              byte
	MqttControlPublishQoS                   byte
	MqttDataSubscriptionQoS                 byte
	MqttDataPublishQoS                      byte
	MqttDisconnectQuiesceTime               uint
	MqttMessageDispatcherConcurrencyLimit   int
	InvalidHandshakeReconnectDelay          int
	KafkaBrokers                            []string
	ClientIdToAccountIdImpl                 string
	ClientIdToAccountIdConfigFile           string
	ClientIdToAccountIdDefaultAccountId     string
	ConnectionDatabaseImpl                  string
	ConnectionDatabaseHost                  string
	ConnectionDatabasePort                  int
	ConnectionDatabaseUser                  string
	ConnectionDatabasePassword              string
	ConnectionDatabaseName                  string
	ConnectionDatabaseSslMode               string
	ConnectionDatabaseSslRootCert           string
	ConnectionDatabaseSqliteFile            string
	ConnectionDatabaseQueryTimeout          time.Duration
	AuthGatewayUrl                          string
	AuthGatewayHttpClientTimeout            time.Duration
	ConnectedClientRecorderImpl             string
	InventoryKafkaBrokers                   []string
	InventoryKafkaTopic                     string
	InventoryKafkaBatchSize                 int
	InventoryKafkaBatchBytes                int
	InventoryStaleTimestampOffset           time.Duration
	InventoryStaleTimestampUpdaterChunkSize int
	InventoryReporterName                   string
	SourcesRecorderImpl                     string
	SourcesBaseUrl                          string
	SourcesHttpClientTimeout                time.Duration
	JwtTokenExpiry                          int
	JwtPrivateKeyFile                       string
	JwtPublicKeyFile                        string
	RhcMessageKafkaBrokers                  []string
	RhcMessageKafkaTopic                    string
	RhcMessageKafkaBatchSize                int
	RhcMessageKafkaBatchBytes               int
	RhcMessageKafkaConsumerGroup            string
	PendoApiEndpoint                        string
	PendoRequestTimeout                     time.Duration
	PendoIntegrationKey                     string
	PendoRequestSize                        int
}

func (c Config) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s\n", URL_PATH_PREFIX, c.UrlPathPrefix)
	fmt.Fprintf(&b, "%s: %s\n", URL_APP_NAME, c.UrlAppName)
	fmt.Fprintf(&b, "%s: %s\n", URL_BASE_PATH, c.UrlBasePath)
	fmt.Fprintf(&b, "%s: %s\n", OPENAPI_SPEC_FILE_PATH, c.OpenApiSpecFilePath)
	fmt.Fprintf(&b, "%s: %s\n", HTTP_SHUTDOWN_TIMEOUT, c.HttpShutdownTimeout)
	fmt.Fprintf(&b, "%s: %t\n", PROFILE, c.Profile)
	fmt.Fprintf(&b, "%s: %s\n", MQTT_BROKER_ADDRESS, c.MqttBrokerAddress)
	fmt.Fprintf(&b, "%s: %s\n", MQTT_CLIENT_ID, c.MqttClientId)
	fmt.Fprintf(&b, "%s: %v\n", MQTT_USE_HOSTNAME_AS_CLIENT_ID, c.MqttUseHostnameAsClientId)
	fmt.Fprintf(&b, "%s: %v\n", MQTT_CLEAN_SESSION, c.MqttCleanSession)
	fmt.Fprintf(&b, "%s: %v\n", MQTT_RESUME_SUBS, c.MqttResumeSubs)
	fmt.Fprintf(&b, "%s: %s\n", MQTT_BROKER_TLS_CERT_FILE, c.MqttBrokerTlsCertFile)
	fmt.Fprintf(&b, "%s: %s\n", MQTT_BROKER_TLS_KEY_FILE, c.MqttBrokerTlsKeyFile)
	fmt.Fprintf(&b, "%s: %s\n", MQTT_BROKER_TLS_CA_CERT_FILE, c.MqttBrokerTlsCACertFile)
	fmt.Fprintf(&b, "%s: %v\n", MQTT_BROKER_TLS_SKIP_VERIFY, c.MqttBrokerTlsSkipVerify)
	fmt.Fprintf(&b, "%s: %s\n", MQTT_BROKER_JWT_GENERATOR_IMPL, c.MqttBrokerJwtGeneratorImpl)
	fmt.Fprintf(&b, "%s: %s\n", MQTT_BROKER_JWT_FILE, c.MqttBrokerJwtFile)
	fmt.Fprintf(&b, "%s: %s\n", MQTT_TOPIC_PREFIX, c.MqttTopicPrefix)
	fmt.Fprintf(&b, "%s: %d\n", MQTT_CONTROL_SUBSCRIPTION_QOS, c.MqttControlSubscriptionQoS)
	fmt.Fprintf(&b, "%s: %d\n", MQTT_CONTROL_PUBLISH_QOS, c.MqttControlPublishQoS)
	fmt.Fprintf(&b, "%s: %d\n", MQTT_DATA_SUBSCRIPTION_QOS, c.MqttDataSubscriptionQoS)
	fmt.Fprintf(&b, "%s: %d\n", MQTT_DATA_PUBLISH_QOS, c.MqttDataPublishQoS)
	fmt.Fprintf(&b, "%s: %d\n", MQTT_DISCONNECT_QUIESCE_TIME, c.MqttDisconnectQuiesceTime)
	fmt.Fprintf(&b, "%s: %d\n", MQTT_MESSAGE_DISPATCHER_CONCURRENCY_LIMIT, c.MqttMessageDispatcherConcurrencyLimit)
	fmt.Fprintf(&b, "%s: %d\n", INVALID_HANDSHAKE_RECONNECT_DELAY, c.InvalidHandshakeReconnectDelay)
	fmt.Fprintf(&b, "%s: %s\n", CLIENT_ID_TO_ACCOUNT_ID_IMPL, c.ClientIdToAccountIdImpl)
	fmt.Fprintf(&b, "%s: %s\n", CLIENT_ID_TO_ACCOUNT_ID_CONFIG_FILE, c.ClientIdToAccountIdConfigFile)
	fmt.Fprintf(&b, "%s: %s\n", CLIENT_ID_TO_ACCOUNT_ID_DEFAULT_ACCOUNT_ID, c.ClientIdToAccountIdDefaultAccountId)
	fmt.Fprintf(&b, "%s: %s\n", CONNECTION_DATABASE_IMPL, c.ConnectionDatabaseImpl)
	fmt.Fprintf(&b, "%s: %s\n", CONNECTION_DATABASE_HOST, c.ConnectionDatabaseHost)
	fmt.Fprintf(&b, "%s: %d\n", CONNECTION_DATABASE_PORT, c.ConnectionDatabasePort)
	fmt.Fprintf(&b, "%s: %s\n", CONNECTION_DATABASE_USER, c.ConnectionDatabaseUser)
	fmt.Fprintf(&b, "%s: %s\n", CONNECTION_DATABASE_NAME, c.ConnectionDatabaseName)
	fmt.Fprintf(&b, "%s: %s\n", CONNECTION_DATABASE_SSL_MODE, c.ConnectionDatabaseSslMode)
	fmt.Fprintf(&b, "%s: %s\n", CONNECTION_DATABASE_SSL_ROOT_CERT, c.ConnectionDatabaseSslRootCert)
	fmt.Fprintf(&b, "%s: %s\n", CONNECTION_DATABASE_SQLITE_FILE, c.ConnectionDatabaseSqliteFile)
	fmt.Fprintf(&b, "%s: %s\n", CONNECTION_DATABASE_QUERY_TIMEOUT, c.ConnectionDatabaseQueryTimeout)
	fmt.Fprintf(&b, "%s: %s\n", CONNECTED_CLIENT_RECORDER_IMPL, c.ConnectedClientRecorderImpl)
	fmt.Fprintf(&b, "%s: %s\n", INVENTORY_KAFKA_BROKERS, c.InventoryKafkaBrokers)
	fmt.Fprintf(&b, "%s: %s\n", INVENTORY_KAFKA_TOPIC, c.InventoryKafkaTopic)
	fmt.Fprintf(&b, "%s: %d\n", INVENTORY_KAFKA_BATCH_SIZE, c.InventoryKafkaBatchSize)
	fmt.Fprintf(&b, "%s: %d\n", INVENTORY_KAFKA_BATCH_BYTES, c.InventoryKafkaBatchBytes)
	fmt.Fprintf(&b, "%s: %s\n", INVENTORY_STALE_TIMESTAMP_OFFSET, c.InventoryStaleTimestampOffset)
	fmt.Fprintf(&b, "%s: %d\n", INVENTORY_STALE_TIMESTAMP_UPDATER_CHUNK_SIZE, c.InventoryStaleTimestampUpdaterChunkSize)
	fmt.Fprintf(&b, "%s: %s\n", INVENTORY_REPORTER_NAME, c.InventoryReporterName)
	fmt.Fprintf(&b, "%s: %s\n", SOURCES_RECORDER_IMPL, c.SourcesRecorderImpl)
	fmt.Fprintf(&b, "%s: %s\n", SOURCES_BASE_URL, c.SourcesBaseUrl)
	fmt.Fprintf(&b, "%s: %s\n", SOURCES_HTTP_CLIENT_TIMEOUT, c.SourcesHttpClientTimeout)
	fmt.Fprintf(&b, "%s: %d\n", JWT_TOKEN_EXPIRY, c.JwtTokenExpiry)
	fmt.Fprintf(&b, "%s: %s\n", JWT_PRIVATE_KEY_FILE, c.JwtPrivateKeyFile)
	fmt.Fprintf(&b, "%s: %s\n", JWT_PUBLIC_KEY_FILE, c.JwtPublicKeyFile)
	fmt.Fprintf(&b, "%s: %s\n", AUTH_GATEWAY_URL, c.AuthGatewayUrl)
	fmt.Fprintf(&b, "%s: %s\n", AUTH_GATEWAY_HTTP_CLIENT_TIMEOUT, c.AuthGatewayHttpClientTimeout)
	fmt.Fprintf(&b, "%s: %s\n", RHC_MESSAGE_KAFKA_BROKERS, c.RhcMessageKafkaBrokers)
	fmt.Fprintf(&b, "%s: %s\n", RHC_MESSAGE_KAFKA_TOPIC, c.RhcMessageKafkaTopic)
	fmt.Fprintf(&b, "%s: %d\n", RHC_MESSAGE_KAFKA_BATCH_SIZE, c.RhcMessageKafkaBatchSize)
	fmt.Fprintf(&b, "%s: %d\n", RHC_MESSAGE_KAFKA_BATCH_BYTES, c.RhcMessageKafkaBatchBytes)
	fmt.Fprintf(&b, "%s: %s\n", RHC_MESSAGE_KAFKA_CONSUMER_GROUP, c.RhcMessageKafkaConsumerGroup)
	return b.String()
}

func GetConfig() *Config {
	options := viper.New()

	options.SetDefault(URL_PATH_PREFIX, "api")
	options.SetDefault(URL_APP_NAME, "cloud-connector")
	options.SetDefault(OPENAPI_SPEC_FILE_PATH, "/opt/app-root/src/api/api.spec.file")
	options.SetDefault(HTTP_SHUTDOWN_TIMEOUT, 2)
	options.SetDefault(SERVICE_TO_SERVICE_CREDENTIALS, "")
	options.SetDefault(PROFILE, false)
	options.SetDefault(MQTT_BROKER_ADDRESS, MQTT_BROKER_ADDRESS_DEFAULT)
	options.SetDefault(MQTT_CLIENT_ID, "")
	options.SetDefault(MQTT_CLEAN_SESSION, false)
	options.SetDefault(MQTT_RESUME_SUBS, true)
	options.SetDefault(MQTT_BROKER_TLS_SKIP_VERIFY, false)
	options.SetDefault(MQTT_BROKER_JWT_GENERATOR_IMPL, "jwt_file_reader")
	options.SetDefault(MQTT_BROKER_JWT_FILE, "cloud-connector-mqtt-jwt.txt")
	options.SetDefault(MQTT_TOPIC_PREFIX, "redhat")
	options.SetDefault(MQTT_CONTROL_SUBSCRIPTION_QOS, 1)
	options.SetDefault(MQTT_CONTROL_PUBLISH_QOS, 1)
	options.SetDefault(MQTT_DATA_SUBSCRIPTION_QOS, 1)
	options.SetDefault(MQTT_DATA_PUBLISH_QOS, 1)
	options.SetDefault(MQTT_DISCONNECT_QUIESCE_TIME, 1000)
	options.SetDefault(MQTT_MESSAGE_DISPATCHER_CONCURRENCY_LIMIT, 1)
	options.SetDefault(INVALID_HANDSHAKE_RECONNECT_DELAY, 5)
	options.SetDefault(CLIENT_ID_TO_ACCOUNT_ID_IMPL, "config_file_based")
	options.SetDefault(CLIENT_ID_TO_ACCOUNT_ID_CONFIG_FILE, "client_id_to_account_id_map.json")
	options.SetDefault(CLIENT_ID_TO_ACCOUNT_ID_DEFAULT_ACCOUNT_ID, "111000")
	options.SetDefault(CONNECTION_DATABASE_IMPL, "postgres")
	options.SetDefault(CONNECTION_DATABASE_HOST, "localhost")
	options.SetDefault(CONNECTION_DATABASE_PORT, 5432)
	options.SetDefault(CONNECTION_DATABASE_USER, "insights")
	options.SetDefault(CONNECTION_DATABASE_PASSWORD, "insights")
	options.SetDefault(CONNECTION_DATABASE_NAME, "cloud-connector")
	options.SetDefault(CONNECTION_DATABASE_SSL_MODE, "disable")
	options.SetDefault(CONNECTION_DATABASE_SSL_ROOT_CERT, "db_ssl_root_cert.pem")
	options.SetDefault(CONNECTION_DATABASE_SQLITE_FILE, "connections_metadata_sqlite.db")
	options.SetDefault(CONNECTION_DATABASE_QUERY_TIMEOUT, 5)
	options.SetDefault(CONNECTED_CLIENT_RECORDER_IMPL, "fake")
	options.SetDefault(INVENTORY_KAFKA_BROKERS, []string{DEFAULT_KAFKA_BROKER_ADDRESS})
	options.SetDefault(INVENTORY_KAFKA_TOPIC, "platform.inventory.host-ingress-p1")
	options.SetDefault(INVENTORY_KAFKA_BATCH_SIZE, 100)
	options.SetDefault(INVENTORY_KAFKA_BATCH_BYTES, 1048576)
	options.SetDefault(INVENTORY_STALE_TIMESTAMP_OFFSET, 26)
	options.SetDefault(INVENTORY_STALE_TIMESTAMP_UPDATER_CHUNK_SIZE, 100)
	options.SetDefault(INVENTORY_REPORTER_NAME, "cloud-connector")
	options.SetDefault(SOURCES_RECORDER_IMPL, "fake")
	options.SetDefault(SOURCES_BASE_URL, "http://sources-api.sources-ci.svc.cluster.local:8080")
	options.SetDefault(SOURCES_HTTP_CLIENT_TIMEOUT, 5)
	options.SetDefault(JWT_TOKEN_EXPIRY, 1)
	options.SetDefault(JWT_PRIVATE_KEY_FILE, "/etc/jwt/mqtt-private-key.rsa")
	options.SetDefault(JWT_PUBLIC_KEY_FILE, "/etc/jwt/mqtt-public-key.rsa")
	options.SetDefault(AUTH_GATEWAY_URL, "http://apicast.3scale-staging.svc.cluster.local:8890/internal/certauth")
	options.SetDefault(AUTH_GATEWAY_HTTP_CLIENT_TIMEOUT, 15)
	options.SetDefault(RHC_MESSAGE_KAFKA_BROKERS, []string{DEFAULT_KAFKA_BROKER_ADDRESS})
	options.SetDefault(RHC_MESSAGE_KAFKA_TOPIC, RHC_MESSAGE_KAFKA_TOPIC_DEFAULT)
	options.SetDefault(RHC_MESSAGE_KAFKA_BATCH_SIZE, 100)
	options.SetDefault(RHC_MESSAGE_KAFKA_BATCH_BYTES, 1048576)
	options.SetDefault(RHC_MESSAGE_KAFKA_CONSUMER_GROUP, "cloud-connector-rhc-message-consumer")
	options.SetDefault(PENDO_API_ENDPOINT, "https://app.pendo.io/api/v1")
	options.SetDefault(PENDO_REQUEST_TIMEOUT, 5)
	options.SetDefault(PENDO_INTEGRATION_KEY, "")
	options.SetDefault(PENDO_REQUEST_SIZE, 100)

	options.SetEnvPrefix(ENV_PREFIX)
	options.AutomaticEnv()

	config := &Config{
		UrlPathPrefix:                           options.GetString(URL_PATH_PREFIX),
		UrlAppName:                              options.GetString(URL_APP_NAME),
		UrlBasePath:                             buildUrlBasePath(options.GetString(URL_PATH_PREFIX), options.GetString(URL_APP_NAME)),
		OpenApiSpecFilePath:                     options.GetString(OPENAPI_SPEC_FILE_PATH),
		HttpShutdownTimeout:                     options.GetDuration(HTTP_SHUTDOWN_TIMEOUT) * time.Second,
		ServiceToServiceCredentials:             options.GetStringMap(SERVICE_TO_SERVICE_CREDENTIALS),
		Profile:                                 options.GetBool(PROFILE),
		MqttBrokerAddress:                       options.GetString(MQTT_BROKER_ADDRESS),
		MqttClientId:                            options.GetString(MQTT_CLIENT_ID),
		MqttUseHostnameAsClientId:               options.GetBool(MQTT_USE_HOSTNAME_AS_CLIENT_ID),
		MqttCleanSession:                        options.GetBool(MQTT_CLEAN_SESSION),
		MqttResumeSubs:                          options.GetBool(MQTT_RESUME_SUBS),
		MqttBrokerTlsCertFile:                   options.GetString(MQTT_BROKER_TLS_CERT_FILE),
		MqttBrokerTlsKeyFile:                    options.GetString(MQTT_BROKER_TLS_KEY_FILE),
		MqttBrokerTlsCACertFile:                 options.GetString(MQTT_BROKER_TLS_CA_CERT_FILE),
		MqttBrokerTlsSkipVerify:                 options.GetBool(MQTT_BROKER_TLS_SKIP_VERIFY),
		MqttBrokerJwtGeneratorImpl:              options.GetString(MQTT_BROKER_JWT_GENERATOR_IMPL),
		MqttBrokerJwtFile:                       options.GetString(MQTT_BROKER_JWT_FILE),
		MqttTopicPrefix:                         options.GetString(MQTT_TOPIC_PREFIX),
		MqttControlSubscriptionQoS:              byte(options.GetInt(MQTT_CONTROL_SUBSCRIPTION_QOS)),
		MqttControlPublishQoS:                   byte(options.GetInt(MQTT_CONTROL_PUBLISH_QOS)),
		MqttDataSubscriptionQoS:                 byte(options.GetInt(MQTT_DATA_SUBSCRIPTION_QOS)),
		MqttDataPublishQoS:                      byte(options.GetInt(MQTT_DATA_PUBLISH_QOS)),
		MqttDisconnectQuiesceTime:               options.GetUint(MQTT_DISCONNECT_QUIESCE_TIME),
		MqttMessageDispatcherConcurrencyLimit:   options.GetInt(MQTT_MESSAGE_DISPATCHER_CONCURRENCY_LIMIT),
		InvalidHandshakeReconnectDelay:          options.GetInt(INVALID_HANDSHAKE_RECONNECT_DELAY),
		ClientIdToAccountIdImpl:                 options.GetString(CLIENT_ID_TO_ACCOUNT_ID_IMPL),
		ClientIdToAccountIdConfigFile:           options.GetString(CLIENT_ID_TO_ACCOUNT_ID_CONFIG_FILE),
		ClientIdToAccountIdDefaultAccountId:     options.GetString(CLIENT_ID_TO_ACCOUNT_ID_DEFAULT_ACCOUNT_ID),
		ConnectionDatabaseImpl:                  options.GetString(CONNECTION_DATABASE_IMPL),
		ConnectionDatabaseHost:                  options.GetString(CONNECTION_DATABASE_HOST),
		ConnectionDatabasePort:                  options.GetInt(CONNECTION_DATABASE_PORT),
		ConnectionDatabaseUser:                  options.GetString(CONNECTION_DATABASE_USER),
		ConnectionDatabasePassword:              options.GetString(CONNECTION_DATABASE_PASSWORD),
		ConnectionDatabaseName:                  options.GetString(CONNECTION_DATABASE_NAME),
		ConnectionDatabaseSslMode:               options.GetString(CONNECTION_DATABASE_SSL_MODE),
		ConnectionDatabaseSslRootCert:           options.GetString(CONNECTION_DATABASE_SSL_ROOT_CERT),
		ConnectionDatabaseSqliteFile:            options.GetString(CONNECTION_DATABASE_SQLITE_FILE),
		ConnectionDatabaseQueryTimeout:          options.GetDuration(CONNECTION_DATABASE_QUERY_TIMEOUT) * time.Second,
		AuthGatewayUrl:                          options.GetString(AUTH_GATEWAY_URL),
		AuthGatewayHttpClientTimeout:            options.GetDuration(AUTH_GATEWAY_HTTP_CLIENT_TIMEOUT) * time.Second,
		ConnectedClientRecorderImpl:             options.GetString(CONNECTED_CLIENT_RECORDER_IMPL),
		InventoryKafkaBrokers:                   options.GetStringSlice(INVENTORY_KAFKA_BROKERS),
		InventoryKafkaTopic:                     options.GetString(INVENTORY_KAFKA_TOPIC),
		InventoryKafkaBatchSize:                 options.GetInt(INVENTORY_KAFKA_BATCH_SIZE),
		InventoryKafkaBatchBytes:                options.GetInt(INVENTORY_KAFKA_BATCH_BYTES),
		InventoryStaleTimestampOffset:           options.GetDuration(INVENTORY_STALE_TIMESTAMP_OFFSET) * time.Hour,
		InventoryStaleTimestampUpdaterChunkSize: options.GetInt(INVENTORY_STALE_TIMESTAMP_UPDATER_CHUNK_SIZE),
		InventoryReporterName:                   options.GetString(INVENTORY_REPORTER_NAME),
		SourcesRecorderImpl:                     options.GetString(SOURCES_RECORDER_IMPL),
		SourcesBaseUrl:                          options.GetString(SOURCES_BASE_URL),
		SourcesHttpClientTimeout:                options.GetDuration(SOURCES_HTTP_CLIENT_TIMEOUT) * time.Second,
		JwtTokenExpiry:                          options.GetInt(JWT_TOKEN_EXPIRY),
		JwtPrivateKeyFile:                       options.GetString(JWT_PRIVATE_KEY_FILE),
		JwtPublicKeyFile:                        options.GetString(JWT_PUBLIC_KEY_FILE),
		RhcMessageKafkaBrokers:                  options.GetStringSlice(RHC_MESSAGE_KAFKA_BROKERS),
		RhcMessageKafkaTopic:                    options.GetString(RHC_MESSAGE_KAFKA_TOPIC),
		RhcMessageKafkaBatchSize:                options.GetInt(RHC_MESSAGE_KAFKA_BATCH_SIZE),
		RhcMessageKafkaBatchBytes:               options.GetInt(RHC_MESSAGE_KAFKA_BATCH_BYTES),
		RhcMessageKafkaConsumerGroup:            options.GetString(RHC_MESSAGE_KAFKA_CONSUMER_GROUP),
		PendoApiEndpoint:                        options.GetString(PENDO_API_ENDPOINT),
		PendoRequestTimeout:                     options.GetDuration(PENDO_REQUEST_TIMEOUT) * time.Second,
		PendoIntegrationKey:                     options.GetString(PENDO_INTEGRATION_KEY),
		PendoRequestSize:                        options.GetInt(PENDO_REQUEST_SIZE),
	}

	if clowder.IsClowderEnabled() {
		cfg := clowder.LoadedConfig

		fmt.Println("Cloud-Connector is running in a Clowderized environment...overriding configuration!!")

		config.InventoryKafkaBrokers = clowder.KafkaServers
		config.InventoryKafkaTopic = clowder.KafkaTopics["platform.inventory.host-ingress-p1"].Name

		config.RhcMessageKafkaBrokers = clowder.KafkaServers
		config.RhcMessageKafkaTopic = clowder.KafkaTopics[RHC_MESSAGE_KAFKA_TOPIC_DEFAULT].Name

		if config.RhcMessageKafkaTopic == "" {
			fmt.Println("WARNING:  RHC Message kafka topic is not set within clowder!!")
			// FIXME:  this is a hack!!
			config.RhcMessageKafkaTopic = RHC_MESSAGE_KAFKA_TOPIC_DEFAULT
		}

		config.ConnectionDatabaseHost = cfg.Database.Hostname
		config.ConnectionDatabasePort = cfg.Database.Port
		config.ConnectionDatabaseName = cfg.Database.Name
		config.ConnectionDatabaseUser = cfg.Database.Username
		config.ConnectionDatabasePassword = cfg.Database.Password

		config.ConnectionDatabaseSslMode = cfg.Database.SslMode
		if cfg.Database.RdsCa != nil {
			pathToDBCertFile, err := cfg.RdsCa()
			if err != nil {
				panic(err)
			}

			config.ConnectionDatabaseSslRootCert = pathToDBCertFile
		}
	}

	return config
}

func buildUrlBasePath(pathPrefix string, appName string) string {
	return fmt.Sprintf("/%s/%s/v1", pathPrefix, appName)
}
