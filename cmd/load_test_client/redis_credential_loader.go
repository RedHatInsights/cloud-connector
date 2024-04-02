package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
)

func addCredentialsToRedis(credFile string, redisAddr string, compressedFile bool) {

	var rdb *redis.Client

	rdb = createRedisClient(redisAddr)

	if compressedFile {
		readCompressedFile(credFile, rdb)
	} else {
		readFile, err := os.Open(credFile)

		if err != nil {
			fmt.Println(err)
		}

		processCredFile(readFile, rdb)

		readFile.Close()
	}

}

func readCompressedFile(credFile string, rdb *redis.Client) {

	readFile, err := os.Open(credFile)
	if err != nil {
		panic(err)
	}

	dec := base64.NewDecoder(base64.StdEncoding, readFile)
	// read decoded data from dec to res
	res, err := io.ReadAll(dec)

	zipReader, err := zip.NewReader(bytes.NewReader(res), int64(len(res)))
	if err != nil {
		panic(err)
	}

	// Read all the files from zip archive
	for _, zipFile := range zipReader.File {
		fmt.Println("Reading file:", zipFile.Name)

		f, e := zipFile.Open()
		if e != nil {
			panic(err)
		}

		processCredFile(f, rdb)

		f.Close()
	}
}

func processCredFile(readFile io.Reader, rdb *redis.Client) {
	fileScanner := bufio.NewScanner(readFile)

	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		creds := strings.Split(fileScanner.Text(), ",")

		addUserToRedis(rdb, creds[0], creds[1])
	}
}

func addUserToRedis(rdb *redis.Client, username string, password string) {
	fmt.Printf("Adding user (%s) to redis\n", username)

	c := Credentials{username, password}

	msgPayload, _ := json.Marshal(c)

	_, err := rdb.RPush(context.TODO(), "credentials_list", msgPayload).Result()
	if err != nil {
		fmt.Printf("Error adding credentials to list %s - err: %s\n", username, err)
	}
}

func retrieveUserFromRedis(rdb *redis.Client) (string, string, error) {

	credsPayload, err := rdb.LPop(context.TODO(), "credentials_list").Result()
	if err != nil {
		fmt.Printf("Error retrieving credentials from list - err: %s\n", err)
		return "", "", err
	}

	var creds Credentials

	err = json.Unmarshal([]byte(credsPayload), &creds)
	if err != nil {
		fmt.Printf("Unable to unmarshal creds: %s - err: %s\n", credsPayload, err)
		return "", "", err
	}

	return creds.Username, creds.Password, nil
}
