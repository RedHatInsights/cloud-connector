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
		Short: "A brief description of your command",
		Run: func(cmd *cobra.Command, args []string) {
			startMqttConnectionHandler(listenAddr)
		},
	}

	rootCmd.AddCommand(mqttConnectionHandlerCmd)

	mqttConnectionHandlerCmd.Flags().StringVarP(&listenAddr, "listen-addr", "l", ":8081", "Hostname:port")

	return rootCmd
}

func main() {
	cmd := NewRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
