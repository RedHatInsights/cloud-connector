package main

import (
    "context"
    "encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

    "github.com/redis/go-redis/v9"
)


func startRedisBasedTestController(cloudConnectorUrl string, orgId string, accountNumber string) {
    rdb := createRedisClient()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

    numberOfConcurrentCloudConnectorRequest := 10

    clientsToSendMessagesTo := make(chan string, numberOfConcurrentCloudConnectorRequest)

	go watchForConnections(rdb)
    go determineTargetClients(rdb, clientsToSendMessagesTo)
    go sendMessagesToClients(rdb, clientsToSendMessagesTo, cloudConnectorUrl, orgId, accountNumber)

	<-c
	fmt.Println("Client going down...disconnecting from mqtt uncleanly")
}


func watchForConnections(rdb *redis.Client) {

    pubsub := rdb.PSubscribe(context.TODO(), "connections")

    _, _ = pubsub.Receive(context.TODO())

    defer pubsub.Close()

    ch := pubsub.Channel()
    for msg := range ch {
        fmt.Println(msg.Channel, msg.Payload)

        var connection ConnectionEvent

        err := json.Unmarshal([]byte(msg.Payload), &connection)
        if err != nil {
            fmt.Println("Unable to unmarshal message: ", msg.Payload)
        }

		_, err = rdb.ZAdd(context.TODO(), "messages_sent", redis.Z{0, connection.ClientId}).Result()
		if err != nil {
			fmt.Println("Error adding connection to sorted set %s", connection.ClientId)
		}
    }
}


func determineTargetClients(rdb *redis.Client, clientsToSendMessagesTo chan string) {

    var chunkSize int64 = 20 // FIXME: make configurable

    // FIXME:  Polling to get a chunk of clients ...this is kinda gross.  Is there a better way?

    for {
        clientIds, err := rdb.ZRange(context.TODO(), "messages_sent", 0, chunkSize).Result()
		if err != nil {
            fmt.Println("Error retreiving chunk of connected clients from sorted set - err:", err)
		}

        fmt.Println("clients that have been sent the least amount of messages: ", clientIds)

        for _, v := range clientIds {
            clientsToSendMessagesTo <- v
        }

        time.Sleep(10 * time.Second)
    }
}


func sendMessagesToClients(rdb *redis.Client, clientsToSendMessagesTo chan string, cloudConnectorUrl string, orgId string, accountNumber string) {

	identityHeader := buildIdentityHeader(orgId, accountNumber)

    for clientId := range clientsToSendMessagesTo {

        // FIXME: Kinda gross...I want to limit the amount of concurrency here...now its going 
        // to be limited by the size of the buffered channel

        go func(c string) {
            fmt.Println("Sending message to client ", c)
            sendMessageToClient(cloudConnectorUrl, c, identityHeader)

            _, err := rdb.ZIncrBy(context.TODO(), "messages_sent", 1, c).Result()
            if err != nil {
                fmt.Println("Error incrementing score in sorted set for client %s", c)
            }

        }(clientId)
    }
}

