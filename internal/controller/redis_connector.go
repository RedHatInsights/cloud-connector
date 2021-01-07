package controller

import (
	"strings"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
)

var allConnectionsKey = "connections"

func getConnectionKey(account, nodeID string) string {
	return account + ":" + nodeID
}

func getAllConnectionsIndexVal(account, nodeID, hostname string) string {
	return account + ":" + nodeID + ":" + hostname
}

func getAccountIndexVal(nodeID, hostname string) string {
	return nodeID + ":" + hostname
}

func getPodIndexVal(account, nodeID string) string {
	return account + ":" + nodeID
}

func addIndexes(client *redis.Client, account, nodeID, hostname string) {
	client.SAdd(allConnectionsKey, getAllConnectionsIndexVal(account, nodeID, hostname)) // get all connections
	client.SAdd(account, getAccountIndexVal(nodeID, hostname))                           // get all account connections
	client.SAdd(hostname, getPodIndexVal(account, nodeID))                               // get all pod connections
}

func removeIndexes(client *redis.Client, account, nodeID, hostname string) {
	client.SRem(allConnectionsKey, getAllConnectionsIndexVal(account, nodeID, hostname))
	client.SRem(account, getAccountIndexVal(nodeID, hostname))
	client.SRem(hostname, getPodIndexVal(account, nodeID))
}

func ExistsInRedis(client *redis.Client, account, nodeID string) bool {
	return client.Exists(getPodIndexVal(account, nodeID)).Val() != 0
}

func RegisterWithRedis(client *redis.Client, account, nodeID, hostname string) error {
	var res bool
	var regErr error

	logger := logger.Log.WithFields(logrus.Fields{"account": account, "nodeID": nodeID})

	_, err := client.TxPipelined(func(pipe redis.Pipeliner) error {
		res, regErr = client.SetNX(getConnectionKey(account, nodeID), hostname, 0).Result()
		if res {
			addIndexes(client, account, nodeID, hostname)
		}
		return regErr
	})

	if err != nil {
		logRedisError(logger, err)
		return err
	}
	if !res {
		logger.Infof("Connection (%s, %s) already found. Not registering.", account, nodeID)
		return DuplicateConnectionError{}
	}

	logger.Printf("Registered a connection (%s, %s) to Redis", account, nodeID)
	return nil
}

func UnregisterWithRedis(client *redis.Client, account, nodeID, hostname string) {
	logger := logger.Log.WithFields(logrus.Fields{"account": account, "nodeID": nodeID})

	_, err := client.TxPipelined(func(pipe redis.Pipeliner) error {
		client.Del(getConnectionKey(account, nodeID))
		removeIndexes(client, account, nodeID, hostname)
		return nil
	})

	if err != nil {
		logRedisError(logger, err)
		logger.Warn("Error attempting to unregister connection from Redis")
	}
}

func GetRedisConnection(client *redis.Client, account, nodeID string) (string, error) {
	logger := logger.Log.WithFields(logrus.Fields{"account": account, "nodeID": nodeID})

	val, err := client.Get(getConnectionKey(account, nodeID)).Result()
	if err != nil && err != redis.Nil {
		logRedisError(logger, err)
	}

	return val, err
}

func GetRedisConnectionsByAccount(client *redis.Client, account string) (map[string]string, error) {
	logger := logger.Log.WithFields(logrus.Fields{"account": account})

	connectionsMap := make(map[string]string)
	accountConnections, err := client.SMembers(account).Result()
	if err != nil {
		logRedisError(logger, err)
		return connectionsMap, err
	}
	for _, conn := range accountConnections {
		s := strings.Split(conn, ":")
		connectionsMap[s[0]] = s[1]
	}
	return connectionsMap, err
}

func GetRedisConnectionsByHost(client *redis.Client, hostname string) (map[string][]string, error) {
	connectionsMap := make(map[string][]string)
	podConnections, err := client.SMembers(hostname).Result()
	if err != nil {
		logRedisError(logrus.NewEntry(logger.Log), err)
		return connectionsMap, err
	}
	for _, conn := range podConnections {
		s := strings.Split(conn, ":")
		account, nodeID := s[0], s[1]
		if _, exists := connectionsMap[account]; !exists {
			connectionsMap[account] = []string{}
		}
		connectionsMap[account] = append(connectionsMap[account], nodeID)
	}
	return connectionsMap, err
}

func GetAllRedisConnections(client *redis.Client) (map[string]map[string]string, error) {
	connectionsMap := make(map[string]map[string]string)
	allConnections, err := client.SMembers(allConnectionsKey).Result()
	if err != nil {
		logRedisError(logrus.NewEntry(logger.Log), err)
		return connectionsMap, err
	}
	for _, conn := range allConnections {
		s := strings.Split(conn, ":")
		account, nodeID, hostname := s[0], s[1], s[2]
		if _, exists := connectionsMap[account]; !exists {
			connectionsMap[account] = make(map[string]string)
		}
		connectionsMap[account][nodeID] = hostname
	}
	return connectionsMap, err
}

func logRedisError(logger *logrus.Entry, err error) {
	if err != nil && err != redis.Nil {
		metrics.redisConnectionError.Inc()
		logger.WithFields(logrus.Fields{"error": err}).Warn("An error occurred while communicating with redis")
	}
}
