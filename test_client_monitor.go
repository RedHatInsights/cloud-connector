package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
)

func main() {

	cloudConnectorUrl := flag.String("cloud-connector", "http://localhost:10000", "protocol / hostname / port of cloud-connector")
	orgId := flag.String("org-id", "10001", "org-id")
	accountNumber := flag.String("account", "010101", "account number")
	flag.Parse()

	identityHeader := buildIdentityHeader(*orgId, *accountNumber)

	endTest := make(chan struct{})

	startTest(*cloudConnectorUrl, identityHeader, endTest)

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("Blocking, press ctrl+c to continue...")
	<-done // Will block here until user hits ctrl+c

	endTest <- struct{}{}

	time.Sleep(1 * time.Second)
}

func startTest(cloudConnectorUrl string, identityHeader string, endTest chan struct{}) {

	cmd, subProcessOutput, subProcessDied := startTestProcess()

	var messageIdExpected string
	//var lastMessageIdReceived string

	go func() {
		var clientId string
		var stopProcessing bool

		var currentClientStatus string

		ticker := time.NewTicker(5 * time.Second)

		for {
			if stopProcessing {
				return
			}
			select {
			case output := <-subProcessOutput:
				fmt.Println("output: ", output)

				results := strings.Split(strings.Trim(output, "\n"), " ")

				if results[0] == "CONNECTED" {
					clientId = results[1]
					fmt.Println("Client is connected!  ClientId - ", clientId)
					currentClientStatus = "CONNECTED"
					time.Sleep(2 * time.Second)
					verifyClientIsRegistered(cloudConnectorUrl, clientId, identityHeader)
				} else if results[0] == "MESSAGE_RECEIVED" {
					fmt.Println("Client received a message! ", results[1])
					if messageIdExpected != results[1] {
						fmt.Println("ERROR! message recieved doesn't match expected!")
					}
				} else {
					fmt.Println("CLIENT OUTPUT: ", output)
				}

				// if connected start timer to send message connection check messages
				// and start a timer for sending "send message" messages
				// and receving "message received" messages

			case <-ticker.C:
				fmt.Println("ticker !")

				if currentClientStatus == "CONNECTED" {
					msgID, _ := sendMessageToClient(cloudConnectorUrl, clientId, identityHeader)

					// FIXME: COULD BE AN ERROR HERE
					messageIdExpected = msgID.String()
				}

			case <-subProcessDied:
				fmt.Println("subprocess died!")

				currentClientStatus = "DISCONNECTED"

				time.Sleep(2 * time.Second)
				// check cloud-connector for disconnected state

				// stop messaging timers

				verifyClientIsUnregistered(cloudConnectorUrl, clientId, identityHeader)

				fmt.Println("Starting new process")
				cmd, subProcessOutput, subProcessDied = startTestProcess()
				fmt.Println("new process started")

			case <-endTest:
				fmt.Println("Testing is over...")
				fmt.Println("Killing process...")
				if err := cmd.Process.Kill(); err != nil {
					fmt.Println("error killing subprocess:", err)
				}
				fmt.Println("Killed process")
				stopProcessing = true
			}
		}
	}()
}

func startTestProcess() (*exec.Cmd, chan string, chan struct{}) {
	ctx := context.TODO()
	//cmd := exec.CommandContext(ctx, "sh", "HARPERDB/run_192.168.131.16_client.sh")
	//cmd := exec.CommandContext(ctx, "echo", "HARPERDB/run_192.168.131.16_client.sh")
	//cmd := exec.CommandContext(ctx, "echo", "hiya Fred!")
	cmd := exec.CommandContext(ctx, "sh", "test_script.sh")
	//cmd := exec.CommandContext(ctx, "go", "run simple_test_client.go -broker wss://rh-gtm.harperdbcloud.com:443 -cert HARPERDB/192.168.131.16/cert.pem -key HARPERDB/192.168.131.16/key.pem")
	//cmd := exec.CommandContext(ctx, "go", "run simple_test_client.go -broker ssl://localhost:8883 -cert dev/test_client/client-0-cert.pem -key dev/test_client/client-0-key.pem")

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Unable to get stdout pipe")
		panic(err)
	}

	if err = cmd.Start(); err != nil {
		fmt.Println("Unable to start process")
		panic(err)
	}

	subProcessOutput := make(chan string)
	subProcessDied := make(chan struct{})

	go readSubprocessOutput(stdoutPipe, subProcessOutput)
	go notifyOfSubprocessDeath(cmd, subProcessDied)

	return cmd, subProcessOutput, subProcessDied
}

func readSubprocessOutput(stdoutPipe io.ReadCloser, output chan string) {
	fmt.Println("output reader started")
	buf := bufio.NewReader(stdoutPipe)
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			if len(line) > 0 {
				output <- line
			}
			fmt.Println("Error reading from stdout...process probably died: ", err)
			return
		}
		output <- line
	}
	fmt.Println("output reader stopped")
}

func notifyOfSubprocessDeath(cmd *exec.Cmd, subProcessDied chan struct{}) {
	fmt.Println("Process death watcher started!")
	fmt.Println("Waiting for process!")
	err := cmd.Wait()
	fmt.Println("process died!", err)
	subProcessDied <- struct{}{}
	fmt.Println("Process death watcher stopped!")
}

func buildIdentityHeader(orgId string, accountNumber string) string {
	s := fmt.Sprintf(`{"identity": {"account_number": "%s", "org_id": "%s", "internal": {}, "service_account": {"client_id": "0000", "username": "jdoe"}, "type": "Associate"}}`, accountNumber, orgId)
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func verifyMessageWasReceived(messageReceived chan string, expectedMessageId uuid.UUID) {
	fmt.Println("Checking to see if message was received")

	select {
	case receivedMessageId := <-messageReceived:
		fmt.Println("Got message: ", receivedMessageId)
		if receivedMessageId != expectedMessageId.String() {
			fmt.Println("Epic fail...got wrong message!")
		}
	case <-time.After(10 * time.Second):
		fmt.Println("Epic Fail...did not get message after 10 second")
	}
}

func verifyClientIsRegistered(cloudConnectorUrl string, clientId string, identityHeader string) {
	fmt.Printf("Verifying client (%s) is registered with cloud-connector!!", clientId)
	status := getClientStatusFromCloudConnector(cloudConnectorUrl, clientId, identityHeader)
	if status != "connected" {
		fmt.Println("***  ERROR:  status should have been connected")
	}
}

func verifyClientIsUnregistered(cloudConnectorUrl string, clientId string, identityHeader string) {
	fmt.Printf("Verifying client (%s) is unregistered with cloud-connector!!", clientId)
	status := getClientStatusFromCloudConnector(cloudConnectorUrl, clientId, identityHeader)
	if status != "disconnected" {
		fmt.Println("***  ERROR:  status should have been disconnected")
	}
}

func getClientStatusFromCloudConnector(cloudConnectorUrl string, clientId string, identityHeader string) string {
	type connectionStatus struct {
		Status string `json:"status"`
	}

	url := fmt.Sprintf("%s/api/cloud-connector/v2/connections/%s/status", cloudConnectorUrl, clientId)

	req, err := http.NewRequest(http.MethodGet, url, nil)

	req.Close = true

	requestId := fmt.Sprint(time.Now().Unix())

	req.Header.Set("x-rh-identity", identityHeader)
	req.Header.Set("x-rh-insights-request-id", requestId)

	client := http.Client{
		Timeout: 2 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		fmt.Println("res: ", res)
		fmt.Println("err: ", err)
		time.Sleep(2 * time.Second)
		panic(err)
	}

	connStatus := connectionStatus{}
	json.NewDecoder(res.Body).Decode(&connStatus)

	fmt.Println("Connection Status: ", connStatus.Status)
	return connStatus.Status
}

func sendMessageToClient(cloudConnectorUrl string, clientId string, identityHeader string) (uuid.UUID, error) {
	fmt.Printf("Sending message to client (%s)!!", clientId)

	jsonBody := []byte(`{"directive": "imadirective", "payload": "imapayload"}`)
	bodyReader := bytes.NewReader(jsonBody)

	type sendMessageResponse struct {
		Id string `json:"id"`
	}

	url := fmt.Sprintf("%s/api/cloud-connector/v2/connections/%s/message", cloudConnectorUrl, clientId)

	req, err := http.NewRequest(http.MethodPost, url, bodyReader)

	req.Close = true

	requestId := fmt.Sprint(time.Now().Unix())

	fmt.Println("request id:", requestId)

	req.Header.Set("x-rh-identity", identityHeader)
	req.Header.Set("x-rh-insights-request-id", requestId)

	client := http.Client{
		Timeout: 2 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		fmt.Println("res: ", res)
		fmt.Println("err: ", err)
		time.Sleep(2 * time.Second)

		panic(err)
	}

	sendMsgResponse := sendMessageResponse{}
	json.NewDecoder(res.Body).Decode(&sendMsgResponse)

	fmt.Println("Message response: ", sendMsgResponse)

	return uuid.Parse(sendMsgResponse.Id)
}

func generateUUID() string {
	id, _ := uuid.NewUUID()
	return id.String()
}
