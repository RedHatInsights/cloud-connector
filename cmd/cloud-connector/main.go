package main

import (
	"os"

	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {

	var listenAddr string
	var excludeAccounts string

	// rootCmd represents the base command when called without any subcommands
	var rootCmd = &cobra.Command{
		Use: "cloud-connector",
	}

	// mqttConnectionHandlerCmd represents the mqttConnectionHandler command
	var mqttConnectionHandlerCmd = &cobra.Command{
		Use:   "mqtt_connection_handler",
		Short: "MQTT Connection Handler",
		Run: func(cmd *cobra.Command, args []string) {
			startMqttConnectionHandler(listenAddr)
		},
	}

	var inventoryStaleTimestampeUpdaterCmd = &cobra.Command{
		Use:   "inventory_stale_timestamp_updater",
		Short: "Inventory Stale Timestamp Updater",
		Run: func(cmd *cobra.Command, args []string) {
			startInventoryStaleTimestampUpdater()
		},
	}

	var connectedAccountReportCmd = &cobra.Command{
		Use:   "connection_count_per_account_reporter",
		Short: "Generate a report on the number of connections per account",
		Run: func(cmd *cobra.Command, args []string) {
			startConnectedAccountReport(excludeAccounts)
		},
	}

	rootCmd.AddCommand(mqttConnectionHandlerCmd)
	mqttConnectionHandlerCmd.Flags().StringVarP(&listenAddr, "listen-addr", "l", ":8081", "Hostname:port")

	rootCmd.AddCommand(inventoryStaleTimestampeUpdaterCmd)

	rootCmd.AddCommand(connectedAccountReportCmd)
	connectedAccountReportCmd.Flags().StringVarP(&excludeAccounts, "exclude-accounts", "e", "477931,6089719,540155", "477931,6089719,540155")

	return rootCmd
}

func main() {
	cmd := NewRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
