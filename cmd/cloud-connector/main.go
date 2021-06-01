package main

import (
	"os"

	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {

	var listenAddr string

	// rootCmd represents the base command when called without any subcommands
	var rootCmd = &cobra.Command{
		Use: "cloud-connector",
	}

	var mqttMessageConsumerCmd = &cobra.Command{
		Use:   "mqtt_message_consumer",
		Short: "Run the mqtt message consumer",
		Run: func(cmd *cobra.Command, args []string) {
			startMqttMessageConsumer(listenAddr)
		},
	}
	mqttMessageConsumerCmd.Flags().StringVarP(&listenAddr, "listen-addr", "l", ":8081", "Hostname:port")

	var kafkaMessageConsumerCmd = &cobra.Command{
		Use:   "kafka_message_consumer",
		Short: "Run the kafka message consumer",
		Run: func(cmd *cobra.Command, args []string) {
			startKafkaMessageConsumer(listenAddr)
		},
	}
	kafkaMessageConsumerCmd.Flags().StringVarP(&listenAddr, "listen-addr", "l", ":8081", "Hostname:port")

	var inventoryStaleTimestampeUpdaterCmd = &cobra.Command{
		Use:   "inventory_stale_timestamp_updater",
		Short: "Run the Inventory stale timestamp updater",
		Run: func(cmd *cobra.Command, args []string) {
			startInventoryStaleTimestampUpdater()
		},
	}

	var apiServerCmd = &cobra.Command{
		Use:   "api_server",
		Short: "Run the Cloud-Connector API Server",
		Run: func(cmd *cobra.Command, args []string) {
			startCloudConnectorApiServer(listenAddr)
		},
	}
	apiServerCmd.Flags().StringVarP(&listenAddr, "listen-addr", "l", ":8081", "Hostname:port")

	rootCmd.AddCommand(mqttMessageConsumerCmd)
	rootCmd.AddCommand(inventoryStaleTimestampeUpdaterCmd)
	rootCmd.AddCommand(apiServerCmd)

	return rootCmd
}

func main() {
	cmd := NewRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
