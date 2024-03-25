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

func retrieveUserFromRedis(rdb *redis.Client) (string, string, error) {

    credsPayload, err := rdb.LPop(context.TODO(), "credentials_list").Result()
    if err != nil {
        fmt.Println("Error retrieving credentials from list - err: %s", err)
        return "", "", err
    }

    var creds Credentials

    err = json.Unmarshal([]byte(credsPayload), &creds)
    if err != nil {
        fmt.Println("Unable to unmarshal creds: ", credsPayload, " - err:", err)
        return "", "", err
    }

    return creds.Username, creds.Password, nil
}
