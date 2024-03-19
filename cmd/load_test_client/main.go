package main

import (
	"os"

	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {

	var cloudConnectorUrl string
    var orgId string
    var account string
    var numberOfClients int

	// rootCmd represents the base command when called without any subcommands
	var rootCmd = &cobra.Command{
		Use: "load-tester",
	}

	var controllerCmd = &cobra.Command{
		Use:   "controller",
		Short: "controller",
		Run: func(cmd *cobra.Command, args []string) {
			startController(cloudConnectorUrl, orgId, account, numberOfClients)
		},
	}
    controllerCmd.Flags().StringVarP(&cloudConnectorUrl, "cloud-connector", "C", "http://localhost:8081", "cloud-connector url")
    controllerCmd.Flags().StringVarP(&orgId, "org-id", "O", "10001", "org-id connections belong to")
    controllerCmd.Flags().StringVarP(&account, "account", "A", "010101", "account number")
    controllerCmd.Flags().IntVar(&numberOfClients, "number-of-clients", 10, "number of clients to spawn")


    var broker string
    var certFile string
    var keyFile string

	var mqttClientCmd = &cobra.Command{
		Use:   "mqtt_client",
		Short: "mqtt_client",
		Run: func(cmd *cobra.Command, args []string) {
            startLoadTestClient(broker, certFile, keyFile)
		},
	}
    mqttClientCmd.Flags().StringVarP(&broker, "broker", "b", "ssl://localhost:8883", "broker url")
    mqttClientCmd.Flags().StringVarP(&certFile, "cert-file", "c", "dev/test_client/client-0-cert.pem", "path to cert")
    mqttClientCmd.Flags().StringVarP(&keyFile, "key-file", "k", "dev/test_client/client-0-key.pem", "path to key")

	rootCmd.AddCommand(controllerCmd)
	rootCmd.AddCommand(mqttClientCmd)

	return rootCmd
}

func main() {
	cmd := NewRootCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
