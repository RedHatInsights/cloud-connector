package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	ENV_PREFIX = "CLOUD_CONNECTOR"

	URL_APP_NAME                               = "URL_App_Name"
	URL_PATH_PREFIX                            = "URL_Path_Prefix"
	URL_BASE_PATH                              = "URL_Base_Path"
	OPENAPI_SPEC_FILE_PATH                     = "OpenAPI_Spec_File_Path"
	HTTP_SHUTDOWN_TIMEOUT                      = "HTTP_Shutdown_Timeout"
	SERVICE_TO_SERVICE_CREDENTIALS             = "Service_To_Service_Credentials"
	PROFILE                                    = "Enable_Profile"
	BROKERS                                    = "Kafka_Brokers"
	JOBS_TOPIC                                 = "Kafka_Jobs_Topic"
	JOBS_GROUP_ID                              = "Kafka_Jobs_Group_Id"
	RESPONSES_TOPIC                            = "Kafka_Responses_Topic"
	RESPONSES_BATCH_SIZE                       = "Kafka_Responses_Batch_Size"
	RESPONSES_BATCH_BYTES                      = "Kafka_Responses_Batch_Bytes"
	DEFAULT_BROKER_ADDRESS                     = "kafka:29092"
	CLIENT_ID_TO_ACCOUNT_ID_IMPL               = "Client_Id_To_Account_Id_Impl"
	CLIENT_ID_TO_ACCOUNT_ID_CONFIG_FILE        = "Client_Id_To_Account_Id_Config_File"
	CLIENT_ID_TO_ACCOUNT_ID_DEFAULT_ACCOUNT_ID = "Client_Id_To_Account_Id_Default_Account_Id"
)

type Config struct {
	UrlAppName                          string
	UrlPathPrefix                       string
	UrlBasePath                         string
	OpenApiSpecFilePath                 string
	HttpShutdownTimeout                 time.Duration
	ServiceToServiceCredentials         map[string]interface{}
	Profile                             bool
	KafkaBrokers                        []string
	KafkaJobsTopic                      string
	KafkaResponsesTopic                 string
	KafkaResponsesBatchSize             int
	KafkaResponsesBatchBytes            int
	KafkaGroupID                        string
	ClientIdToAccountIdImpl             string
	ClientIdToAccountIdConfigFile       string
	ClientIdToAccountIdDefaultAccountId string
}

func (c Config) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s\n", URL_PATH_PREFIX, c.UrlPathPrefix)
	fmt.Fprintf(&b, "%s: %s\n", URL_APP_NAME, c.UrlAppName)
	fmt.Fprintf(&b, "%s: %s\n", URL_BASE_PATH, c.UrlBasePath)
	fmt.Fprintf(&b, "%s: %s\n", OPENAPI_SPEC_FILE_PATH, c.OpenApiSpecFilePath)
	fmt.Fprintf(&b, "%s: %s\n", HTTP_SHUTDOWN_TIMEOUT, c.HttpShutdownTimeout)
	fmt.Fprintf(&b, "%s: %t\n", PROFILE, c.Profile)
	fmt.Fprintf(&b, "%s: %s\n", BROKERS, c.KafkaBrokers)
	fmt.Fprintf(&b, "%s: %s\n", JOBS_TOPIC, c.KafkaJobsTopic)
	fmt.Fprintf(&b, "%s: %s\n", RESPONSES_TOPIC, c.KafkaResponsesTopic)
	fmt.Fprintf(&b, "%s: %d\n", RESPONSES_BATCH_SIZE, c.KafkaResponsesBatchSize)
	fmt.Fprintf(&b, "%s: %d\n", RESPONSES_BATCH_BYTES, c.KafkaResponsesBatchBytes)
	fmt.Fprintf(&b, "%s: %s\n", JOBS_GROUP_ID, c.KafkaGroupID)
	fmt.Fprintf(&b, "%s: %s\n", CLIENT_ID_TO_ACCOUNT_ID_IMPL, c.ClientIdToAccountIdImpl)
	fmt.Fprintf(&b, "%s: %s\n", CLIENT_ID_TO_ACCOUNT_ID_CONFIG_FILE, c.ClientIdToAccountIdConfigFile)
	fmt.Fprintf(&b, "%s: %s\n", CLIENT_ID_TO_ACCOUNT_ID_DEFAULT_ACCOUNT_ID, c.ClientIdToAccountIdDefaultAccountId)

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
	options.SetDefault(BROKERS, []string{DEFAULT_BROKER_ADDRESS})
	options.SetDefault(JOBS_TOPIC, "platform.receptor-controller.jobs")
	options.SetDefault(RESPONSES_TOPIC, "platform.receptor-controller.responses")
	options.SetDefault(RESPONSES_BATCH_SIZE, 100)
	options.SetDefault(RESPONSES_BATCH_BYTES, 1048576)
	options.SetDefault(JOBS_GROUP_ID, "cloud-connector-consumer")

	options.SetDefault(CLIENT_ID_TO_ACCOUNT_ID_IMPL, "config_file_based")
	options.SetDefault(CLIENT_ID_TO_ACCOUNT_ID_CONFIG_FILE, "client_id_to_account_id_map.json")
	options.SetDefault(CLIENT_ID_TO_ACCOUNT_ID_DEFAULT_ACCOUNT_ID, "111000")

	options.SetEnvPrefix(ENV_PREFIX)
	options.AutomaticEnv()

	return &Config{
		UrlPathPrefix:                       options.GetString(URL_PATH_PREFIX),
		UrlAppName:                          options.GetString(URL_APP_NAME),
		UrlBasePath:                         buildUrlBasePath(options.GetString(URL_PATH_PREFIX), options.GetString(URL_APP_NAME)),
		OpenApiSpecFilePath:                 options.GetString(OPENAPI_SPEC_FILE_PATH),
		HttpShutdownTimeout:                 options.GetDuration(HTTP_SHUTDOWN_TIMEOUT) * time.Second,
		ServiceToServiceCredentials:         options.GetStringMap(SERVICE_TO_SERVICE_CREDENTIALS),
		Profile:                             options.GetBool(PROFILE),
		KafkaBrokers:                        options.GetStringSlice(BROKERS),
		KafkaJobsTopic:                      options.GetString(JOBS_TOPIC),
		KafkaResponsesTopic:                 options.GetString(RESPONSES_TOPIC),
		KafkaResponsesBatchSize:             options.GetInt(RESPONSES_BATCH_SIZE),
		KafkaResponsesBatchBytes:            options.GetInt(RESPONSES_BATCH_BYTES),
		KafkaGroupID:                        options.GetString(JOBS_GROUP_ID),
		ClientIdToAccountIdImpl:             options.GetString(CLIENT_ID_TO_ACCOUNT_ID_IMPL),
		ClientIdToAccountIdConfigFile:       options.GetString(CLIENT_ID_TO_ACCOUNT_ID_CONFIG_FILE),
		ClientIdToAccountIdDefaultAccountId: options.GetString(CLIENT_ID_TO_ACCOUNT_ID_DEFAULT_ACCOUNT_ID),
	}
}

func buildUrlBasePath(pathPrefix string, appName string) string {
	return fmt.Sprintf("/%s/%s/v1", pathPrefix, appName)
}
