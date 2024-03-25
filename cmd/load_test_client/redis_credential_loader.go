package main

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "os"
    "strings" 

    "github.com/redis/go-redis/v9"
)

func addCredentialsToRedis(credFile string) {

    var rdb *redis.Client

    rdb = createRedisClient()

    readFile, err := os.Open(credFile)

    if err != nil {
        fmt.Println(err)
    }
    fileScanner := bufio.NewScanner(readFile)

    fileScanner.Split(bufio.ScanLines)

    for fileScanner.Scan() {
        creds := strings.Split(fileScanner.Text(), ",")

        addUserToRedis(rdb, creds[0], creds[1])
    }

    readFile.Close()

    /*
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
    */
}

func addUserToRedis(rdb *redis.Client, username string, password string) {
    fmt.Printf("Adding user (%s) to redis\n", username)

    c := Credentials{username, password}

    msgPayload, _ := json.Marshal(c)

    _, err := rdb.RPush(context.TODO(), "credentials_list", msgPayload).Result()
    if err != nil {
        fmt.Println("Error adding credentials to list %s - err: %s", username, err)
    }
}



