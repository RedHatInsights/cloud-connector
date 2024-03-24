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

	go watchForConnections(rdb)
    go sendMessagesToConnectedClients(rdb)

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


func sendMessagesToConnectedClients(rdb *redis.Client) {

    // FIXME:  Polling to get a chunk of clients ...this is kinda gross.  Is there a better way?

    for {
        clientIds, err := rdb.ZRange(context.TODO(), "messages_sent", 0, -1).Result()
		if err != nil {
            fmt.Println("Error retreiving chunk of connected clients from sorted set - err:", err)
		}

        fmt.Println("clients that have been sent the least amount of messages: ", clientIds)

        time.Sleep(10 * time.Second)
    }
}
