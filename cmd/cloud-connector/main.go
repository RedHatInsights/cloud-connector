package main

import (
	"context"
	"fmt"
	"os"

	cr "github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
	pt "github.com/RedHatInsights/cloud-connector/internal/pendo_transmitter"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {

	var listenAddr string
	var excludeAccounts string
	var reportMode string

	// rootCmd represents the base command when called without any subcommands
	var rootCmd = &cobra.Command{
		Use: "cloud-connector",
	}

	var connectionCountCmd = &cobra.Command{
		Use:   "publish_connection_count",
		Short: "Gets the connection count from the databaseand pushes it to Prometheus",
		Run: func(cmd *cobra.Command, args []string) {
			startConnectionCount(listenAddr)
		},
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

	var tokenGeneratorServerCmd = &cobra.Command{
		Use:   "token_server",
		Short: "Run the Cloud-Connector Token Generator Server",
		Run: func(cmd *cobra.Command, args []string) {
			startCloudConnectorTokenGeneratorServer(listenAddr)
		},
	}
	tokenGeneratorServerCmd.Flags().StringVarP(&listenAddr, "listen-addr", "l", ":8081", "Hostname:port")

	var connectedAccountReportCmd = &cobra.Command{
		Use:   "connection_count_per_account_reporter",
		Short: "Generate a report on the number of connections per account",
		Run: func(cmd *cobra.Command, args []string) {
			switch reportMode {
			case "pendo":
				pt.PendoReporter(excludeAccounts)
			case "stdout":
				cr.StartConnectedAccountReport(excludeAccounts, stdoutConnectionCountProcessor)
			}
		},
	}
	connectedAccountReportCmd.Flags().StringVarP(&excludeAccounts, "exclude-accounts", "e", "477931,6089719,540155", "477931,6089719,540155")
	connectedAccountReportCmd.Flags().StringVarP(&reportMode, "report-exporter", "r", "stdout", "Report export method - stdout/pendo")

	rootCmd.AddCommand(mqttMessageConsumerCmd)
	rootCmd.AddCommand(inventoryStaleTimestampeUpdaterCmd)
	rootCmd.AddCommand(apiServerCmd)
	rootCmd.AddCommand(tokenGeneratorServerCmd)
	rootCmd.AddCommand(kafkaMessageConsumerCmd)
	rootCmd.AddCommand(connectedAccountReportCmd)
	rootCmd.AddCommand(connectionCountCmd)

	return rootCmd
}

func stdoutConnectionCountProcessor(ctx context.Context, account domain.AccountID, count int) error {
	fmt.Printf("%s - %d\n", account, count)
	return nil
}

func main() {
	logger.InitLogger()
	defer logger.FlushLogger()

	cmd := NewRootCommand()
	if err := cmd.Execute(); err != nil {
		logger.FlushLogger()
		os.Exit(1)
	}
}
