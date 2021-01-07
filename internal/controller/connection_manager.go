package controller

import (
	"context"
	"sync"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/sirupsen/logrus"
)

type DuplicateConnectionError struct {
}

func (d DuplicateConnectionError) Error() string {
	return "duplicate node id"
}

type ConnectionRegistrar interface {
	Register(ctx context.Context, account string, node_id string, client Receptor) error
	Unregister(ctx context.Context, account string, node_id string)
}

type ConnectionLocator interface {
	GetConnection(ctx context.Context, account string, node_id string) Receptor
	GetConnectionsByAccount(ctx context.Context, account string) map[string]Receptor
	GetAllConnections(ctx context.Context) map[string]map[string]Receptor
}

type LocalConnectionManager struct {
	connections map[string]map[string]Receptor
	sync.RWMutex
}

func NewLocalConnectionManager() *LocalConnectionManager {
	return &LocalConnectionManager{
		connections: make(map[string]map[string]Receptor),
	}
}

func (cm *LocalConnectionManager) Register(ctx context.Context, account string, node_id string, client Receptor) error {
	cm.Lock()
	defer cm.Unlock()
	_, exists := cm.connections[account]
	if exists == true { // checking connection locally
		_, exists = cm.connections[account][node_id]
		if exists == true {
			logger := logger.Log.WithFields(logrus.Fields{"account": account, "node_id": node_id})
			logger.Warn("Attempting to register duplicate connection")
			return DuplicateConnectionError{}
		}
		cm.connections[account][node_id] = client
	} else {
		cm.connections[account] = make(map[string]Receptor)
		cm.connections[account][node_id] = client
	}

	logger.Log.Printf("Registered a connection (%s, %s)", account, node_id)
	return nil
}

func (cm *LocalConnectionManager) Unregister(ctx context.Context, account string, node_id string) {
	cm.Lock()
	defer cm.Unlock()
	_, exists := cm.connections[account]
	if exists == false {
		return
	}
	delete(cm.connections[account], node_id)

	if len(cm.connections[account]) == 0 {
		delete(cm.connections, account)
	}

	logger.Log.Printf("Unregistered a connection (%s, %s)", account, node_id)
}

func (cm *LocalConnectionManager) GetConnection(ctx context.Context, account string, node_id string) Receptor {
	var conn Receptor

	cm.RLock()
	defer cm.RUnlock()
	_, exists := cm.connections[account]
	if exists == false {
		return nil
	}

	conn, exists = cm.connections[account][node_id]
	if exists == false {
		return nil
	}

	return conn
}

func (cm *LocalConnectionManager) GetConnectionsByAccount(ctx context.Context, account string) map[string]Receptor {
	cm.RLock()
	defer cm.RUnlock()

	connectionsPerAccount := make(map[string]Receptor)

	_, exists := cm.connections[account]
	if exists == false {
		return connectionsPerAccount
	}

	for k, v := range cm.connections[account] {
		connectionsPerAccount[k] = v
	}

	return connectionsPerAccount
}

func (cm *LocalConnectionManager) GetAllConnections(ctx context.Context) map[string]map[string]Receptor {
	cm.RLock()
	defer cm.RUnlock()

	connectionMap := make(map[string]map[string]Receptor)

	for accountNumber, accountMap := range cm.connections {
		connectionMap[accountNumber] = make(map[string]Receptor)
		for nodeID, receptorObj := range accountMap {
			connectionMap[accountNumber][nodeID] = receptorObj
		}
	}

	return connectionMap
}
