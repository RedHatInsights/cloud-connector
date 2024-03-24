package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

func startController(cloudConnectorUrl string, orgId string, accountNumber string, numberOfClients int) {
	rand.Seed(time.Now().UnixNano())

	identityHeader := buildIdentityHeader(orgId, accountNumber)

	stopTest := make(chan struct{})
	testGoRoutineDone := make(chan struct{})

	connectedClients := make(map[string]bool)
	clientConnected := recordClientConnected(connectedClients)

	messagesSent := make(map[string]map[string]bool)
	messageSent := recordMessageSent(messagesSent)

	messagesReceived := make(map[string]map[string]bool)
	messageReceived := recordMessageReceived(messagesReceived)

	for i := 0; i < numberOfClients; i++ {
		err := startTest(cloudConnectorUrl, identityHeader, stopTest, testGoRoutineDone, clientConnected, messageSent, messageReceived)

		if err != nil {
			panic(err)
		}
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("Blocking, press ctrl+c to continue...")

	// Will block here until user hits ctrl+c or the test fails
	select {
	case <-done:
		go func() { // Very gross
			stopTest <- struct{}{}
		}()
		break
	case <-testGoRoutineDone:
		break
	}

	time.Sleep(1 * time.Second)

	fmt.Println("connected clients:", connectedClients)
	fmt.Println("number of connected clients:", len(connectedClients))

	calculateMissingMessages(messagesSent, messagesReceived)
}

func startTest(cloudConnectorUrl string, identityHeader string, stopTest chan struct{}, testGoRoutineDone chan struct{}, clientConnected func(string) error, messageSent func(string, string) error, messageReceived func(string, string) error) error {

	subProcessOutput := make(chan string)
	subProcessDied := make(chan struct{})

	err := startTestProcess(subProcessOutput, subProcessDied, stopTest)

	if err != nil {
		return err
	}

	var messageIdExpected string

	go func() {
		var clientId string
		var stopProcessing bool

		var currentClientStatus string

		randomMessageSendingTime := rand.Intn(10)
		if randomMessageSendingTime == 0 {
			randomMessageSendingTime = 1
		}

		fmt.Printf("Sleep time: %d\n", randomMessageSendingTime)
		ticker := time.NewTicker(time.Duration(randomMessageSendingTime) * time.Second)

		for {
			if stopProcessing {
				break
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

					clientIsConnected := verifyClientIsRegistered(cloudConnectorUrl, clientId, identityHeader)

					if clientIsConnected == false {
						stopProcessing = true
						break
					}

					clientConnected(clientId)

				} else if results[0] == "MESSAGE_RECEIVED" {
					messageIdReceived := results[1]

					messageReceived(clientId, messageIdReceived)
					fmt.Println("Client received the expected message: ", messageIdReceived)

					if messageIdExpected != messageIdReceived {
						fmt.Printf("ERROR! message id recieved (%s) doesn't match expected message id (%s)!\n", messageIdReceived, messageIdExpected)
						stopProcessing = true
					}
				}

			case <-ticker.C:

				if currentClientStatus == "CONNECTED" {
					msgID, _ := sendMessageToClient(cloudConnectorUrl, clientId, identityHeader)

					// FIXME: COULD BE AN ERROR HERE
					messageIdExpected = msgID.String()

					messageSent(clientId, messageIdExpected)
				}

			case <-subProcessDied:
				fmt.Println("MQTT client died!")

				if clientId == "" {
					fmt.Println("Looks like process never cleanly started...stop the tests")
					// Client process must not have started cleanly...stop testing
					stopProcessing = true
					break
				}

				currentClientStatus = "DISCONNECTED"

				time.Sleep(2 * time.Second)

				clientIsDisconnected := verifyClientIsUnregistered(cloudConnectorUrl, clientId, identityHeader)

				if clientIsDisconnected == false {
					stopProcessing = true
				}

				/*
									fmt.Println("Starting new process")
									err = startTestProcess()
					                if err != nil {
					                    stopProcessing = true
					                }
									fmt.Println("new process started")
				*/

				/*
					case <-stopTest:
						fmt.Println("Testing is over...")
						fmt.Println("Killing process...")
						if err := cmd.Process.Kill(); err != nil {
							fmt.Println("error killing subprocess:", err)
						}
						fmt.Println("Killed process")
						stopProcessing = true
				*/
			}
		}

		fmt.Println("Go routine is leaving...")
		testGoRoutineDone <- struct{}{}
		fmt.Println("Go routine is OUT!!")
	}()

	return nil
}

func startTestProcess(subProcessOutput chan string, subProcessDied chan struct{}, stopTest chan struct{}) error {

	ctx := context.TODO()
	cmd := exec.CommandContext(ctx, "sh", "test_script.sh")

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Unable to get stdout pipe")
		return err
	}

	if err = cmd.Start(); err != nil {
		fmt.Println("Unable to start process")
		return err
	}

	go readSubprocessOutput(stdoutPipe, subProcessOutput)
	go notifyOfSubprocessDeath(cmd, subProcessDied)
	go stopProcess(cmd, stopTest)

	return nil
}

func stopProcess(cmd *exec.Cmd, stopTest chan struct{}) {
	<-stopTest
	fmt.Println("Testing is over...")
	fmt.Println("Killing process...")
	if err := cmd.Process.Kill(); err != nil {
		fmt.Println("error killing subprocess:", err)
	}
	fmt.Println("Killed process")
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

func verifyClientIsRegistered(cloudConnectorUrl string, clientId string, identityHeader string) bool {
	fmt.Printf("Verifying client (%s) is registered with cloud-connector!!\n", clientId)
	status := getClientStatusFromCloudConnector(cloudConnectorUrl, clientId, identityHeader)

	if status == "connected" {
		return true
	}

	fmt.Println("***  ERROR:  status should have been connected")
	return false
}

func verifyClientIsUnregistered(cloudConnectorUrl string, clientId string, identityHeader string) bool {
	fmt.Printf("Verifying client (%s) is unregistered with cloud-connector!!\n", clientId)
	status := getClientStatusFromCloudConnector(cloudConnectorUrl, clientId, identityHeader)

	if status == "disconnected" {
		return true
	}

	fmt.Println("***  ERROR:  status should have been disconnected")
	return false
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
	fmt.Printf("Sending message to client (%s)!!\n", clientId)

	jsonBody := []byte(`{"directive": "imadirective", "payload": "imapayload"}`)
	bodyReader := bytes.NewReader(jsonBody)

	type sendMessageResponse struct {
		Id string `json:"id"`
	}

	url := fmt.Sprintf("%s/api/cloud-connector/v2/connections/%s/message", cloudConnectorUrl, clientId)

	req, err := http.NewRequest(http.MethodPost, url, bodyReader)

	req.Close = true

	requestId := fmt.Sprint(time.Now().Unix())

	//fmt.Println("request id:", requestId)

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

	fmt.Println("Message ID to expect: ", sendMsgResponse)

	return uuid.Parse(sendMsgResponse.Id)
}

type clientConnected func(string) error
type messageSent func(string, string) error
type messageReceived func(string, string) error

func recordClientConnected(connectedClients map[string]bool) clientConnected {

	var mutex sync.Mutex

	return func(clientId string) error {
		mutex.Lock()
		connectedClients[clientId] = true
		mutex.Unlock()
		return nil
	}
}

func recordMessageSent(messagesSent map[string]map[string]bool) messageSent {

	var mutex sync.Mutex

	return func(clientId string, messageId string) error {
		mutex.Lock()

		_, exist := messagesSent[clientId]
		if !exist {
			messagesSent[clientId] = make(map[string]bool)
		}

		messagesSent[clientId][messageId] = true

		mutex.Unlock()

		return nil
	}
}

func recordMessageReceived(messagesReceived map[string]map[string]bool) messageReceived {

	var mutex sync.Mutex

	return func(clientId string, messageId string) error {
		mutex.Lock()

		_, exist := messagesReceived[clientId]
		if !exist {
			messagesReceived[clientId] = make(map[string]bool)
		}

		messagesReceived[clientId][messageId] = true

		mutex.Unlock()

		return nil
	}
}

func calculateMissingMessages(messagesSent map[string]map[string]bool, messagesReceived map[string]map[string]bool) {
	for clientId, _ := range messagesSent {
		for messageId, _ := range messagesSent[clientId] {

			if _, ok := messagesReceived[clientId]; !ok {
				fmt.Println("Client did not recieve any messages:", clientId)
				break
			}

			if _, ok := messagesReceived[clientId][messageId]; !ok {
				fmt.Printf("Client %s did not recieve message %s\n", clientId, messageId)
				continue
			}

			delete(messagesReceived[clientId], messageId)

			if len(messagesReceived[clientId]) == 0 {
				delete(messagesReceived, clientId)
			}
		}
	}

	//    fmt.Println("messagesSent:", messagesSent)
	fmt.Println("missing messages:", messagesReceived)
}
