package main

import (
    "github.com/redis/go-redis/v9"
)

func addCredentialsToRedis(rdb *redis.Client) {

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



