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

	rootCmd.AddCommand(mqttConnectionHandlerCmd)

	rootCmd.AddCommand(inventoryStaleTimestampeUpdaterCmd)

	mqttConnectionHandlerCmd.Flags().StringVarP(&listenAddr, "listen-addr", "l", ":8081", "Hostname:port")

	return rootCmd
}

func main() {
	cmd := NewRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
