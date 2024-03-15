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

	stopTest := make(chan struct{})
	testGoRoutineDone := make(chan struct{})

    err := startTest(*cloudConnectorUrl, identityHeader, stopTest, testGoRoutineDone)
    
    if err != nil {
        panic(err)
    }

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("Blocking, press ctrl+c to continue...")

    // Will block here until user hits ctrl+c or the test fails
    select {
    case <-done:
        go func() {  // Very gross
            stopTest <- struct{}{}
        }()
        break
    case <- testGoRoutineDone:
        break
    }

	time.Sleep(1 * time.Second)
}

func startTest(cloudConnectorUrl string, identityHeader string, stopTest chan struct{}, testGoRoutineDone chan struct{}) (error) {

	cmd, subProcessOutput, subProcessDied, err := startTestProcess()

    if err != nil {
        return err
    }

	var messageIdExpected string

	go func() {
		var clientId string
		var stopProcessing bool

		var currentClientStatus string

		ticker := time.NewTicker(5 * time.Second)

		for {
			if stopProcessing {
                break
			}
			select {
			case output := <-subProcessOutput:
				//fmt.Println("output: ", output)

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

				} else if results[0] == "MESSAGE_RECEIVED" {
					messageIdReceived := results[1]

					fmt.Println("Client received the expected message: ", messageIdReceived)

					if messageIdExpected != messageIdReceived {
						fmt.Println("ERROR! message id recieved (%s) doesn't match expected message id (%s)!", messageIdReceived, messageIdExpected)
						stopProcessing = true
					}
				}

			case <-ticker.C:

				if currentClientStatus == "CONNECTED" {
					msgID, _ := sendMessageToClient(cloudConnectorUrl, clientId, identityHeader)

					// FIXME: COULD BE AN ERROR HERE
					messageIdExpected = msgID.String()
				}

			case <-subProcessDied:
				fmt.Println("MQTT client died!")

                if clientId == "" {
                    fmt.Println("Looks like process never cleanly started...stop the tests")
                    // Client process must not have started cleanly...stop testing
                    stopProcessing = true
                    fmt.Println("HERE")
                    break
                    fmt.Println("THERE")
                }

				currentClientStatus = "DISCONNECTED"

				time.Sleep(2 * time.Second)

				clientIsDisconnected := verifyClientIsUnregistered(cloudConnectorUrl, clientId, identityHeader)

				if clientIsDisconnected == false {
					stopProcessing = true
				}

				fmt.Println("Starting new process")
				cmd, subProcessOutput, subProcessDied, err = startTestProcess()
                if err != nil {
                    stopProcessing = true
                }

				fmt.Println("new process started")

			case <-stopTest:
				fmt.Println("Testing is over...")
				fmt.Println("Killing process...")
				if err := cmd.Process.Kill(); err != nil {
					fmt.Println("error killing subprocess:", err)
				}
				fmt.Println("Killed process")
				stopProcessing = true
			}
		}

        fmt.Println("Go routine is leaving...")
        testGoRoutineDone <- struct{}{}
        fmt.Println("Go routine is OUT!!")
	}()

    return nil
}

func startTestProcess() (*exec.Cmd, chan string, chan struct{}, error) {
	ctx := context.TODO()
	//cmd := exec.CommandContext(ctx, "sh", "HARPERDB/run_192.168.131.16_client.sh")
	//cmd := exec.CommandContext(ctx, "echo", "HARPERDB/run_192.168.131.16_client.sh")
	//cmd := exec.CommandContext(ctx, "echo", "hiya Fred!")

	cmd := exec.CommandContext(ctx, "sh", "test_script.sh")

	//cmd := exec.CommandContext(ctx, "go", "run simple_test_client.go -broker wss://rh-gtm.harperdbcloud.com:443 -cert HARPERDB/192.168.131.16/cert.pem -key HARPERDB/192.168.131.16/key.pem")
	//cmd := exec.CommandContext(ctx, "go", "run simple_test_client.go -broker ssl://localhost:8883 -cert dev/test_client/client-0-cert.pem -key dev/test_client/client-0-key.pem")

    /*
CERT=HARPERDB/192.168.131.16/cert.pem
KEY=HARPERDB/192.168.131.16/key.pem
BROKER="wss://rh-gtm.harperdbcloud.com:443"

go run simple_test_client.go -broker $BROKER -cert $CERT -key $KEY
*/

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("Unable to get stdout pipe")
        return nil, nil, nil, err
	}

	if err = cmd.Start(); err != nil {
		fmt.Println("Unable to start process")
        return nil, nil, nil, err
	}

	subProcessOutput := make(chan string)
	subProcessDied := make(chan struct{})

	go readSubprocessOutput(stdoutPipe, subProcessOutput)
	go notifyOfSubprocessDeath(cmd, subProcessDied)

	return cmd, subProcessOutput, subProcessDied, nil
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

func generateUUID() string {
	id, _ := uuid.NewUUID()
	return id.String()
}
