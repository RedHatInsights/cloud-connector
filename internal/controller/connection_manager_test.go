package controller

import (
	"context"
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
)

func init() {
	logger.InitLogger()
}

// FIXME: Move this to a "central" place
type MockReceptor struct {
	NodeID string
}

func (mr *MockReceptor) SendMessage(context.Context, string, string, string, interface{}, interface{}) (*uuid.UUID, error) {
	return nil, nil
}

func (mr *MockReceptor) Close(context.Context) error {
	return nil
}

func TestCheckForLocalConnectionThatDoesNotExist(t *testing.T) {
	var cl ConnectionLocator
	cl = NewLocalConnectionManager()
	receptorConnection := cl.GetConnection(context.TODO(), "not gonna find me", "or me")
	if receptorConnection != nil {
		t.Fatalf("Expected to not find a connection, but a connection was found")
	}
}

func TestCheckForLocalConnectionThatDoesNotExistButAccountExists(t *testing.T) {
	registeredAccount := "123"
	lcm := NewLocalConnectionManager()
	lcm.Register(context.TODO(), registeredAccount, "456", &MockReceptor{})
	receptorConnection := lcm.GetConnection(context.TODO(), registeredAccount, "not gonna find me")
	if receptorConnection != nil {
		t.Fatalf("Expected to not find a connection, but a connection was found")
	}
}

func TestCheckForLocalConnectionThatDoesExist(t *testing.T) {
	mockReceptor := &MockReceptor{}
	cm := NewLocalConnectionManager()
	cm.Register(context.TODO(), "123", "456", mockReceptor)
	receptorConnection := cm.GetConnection(context.TODO(), "123", "456")
	if receptorConnection == nil {
		t.Fatalf("Expected to find a connection, but did not find a connection")
	}

	if mockReceptor != receptorConnection {
		t.Fatalf("Found the wrong connection")
	}
}

func TestRegisterAndUnregisterMultipleLocalConnectionsPerAccount(t *testing.T) {
	accountNumber := "0000001"
	var testReceptors = []struct {
		account  string
		node_id  string
		receptor *MockReceptor
	}{
		{accountNumber, "node-a", &MockReceptor{}},
		{accountNumber, "node-b", &MockReceptor{}},
	}
	cm := NewLocalConnectionManager()
	for _, r := range testReceptors {
		cm.Register(context.TODO(), r.account, r.node_id, r.receptor)

		actualReceptor := cm.GetConnection(context.TODO(), r.account, r.node_id)
		if actualReceptor == nil {
			t.Fatalf("Expected to find a connection, but did not find a connection")
		}

		if r.receptor != actualReceptor {
			t.Fatalf("Found the wrong connection")
		}
	}

	for _, r := range testReceptors {
		cm.Unregister(context.TODO(), r.account, r.node_id)
	}
}

func TestUnregisterLocalConnectionThatDoesNotExist(t *testing.T) {
	cm := NewLocalConnectionManager()
	cm.Unregister(context.TODO(), "not gonna find me", "or me")
}

func TestGetLocalConnectionsByAccount(t *testing.T) {
	accountNumber := "0000001"
	var testReceptors = []struct {
		account  string
		node_id  string
		receptor *MockReceptor
	}{
		{accountNumber, "node-a", &MockReceptor{}},
		{accountNumber, "node-b", &MockReceptor{}},
	}
	cm := NewLocalConnectionManager()
	for _, r := range testReceptors {
		cm.Register(context.TODO(), r.account, r.node_id, r.receptor)
	}

	receptorMap := cm.GetConnectionsByAccount(context.TODO(), accountNumber)
	if len(receptorMap) != len(testReceptors) {
		t.Fatalf("Expected to find %d connections, but found %d connections", len(testReceptors), len(receptorMap))
	}
}

func TestGetLocalConnectionsByAccountWithNoRegisteredReceptors(t *testing.T) {
	cm := NewLocalConnectionManager()
	receptorMap := cm.GetConnectionsByAccount(context.TODO(), "0000001")
	if len(receptorMap) != 0 {
		t.Fatalf("Expected to find 0 connections, but found %d connections", len(receptorMap))
	}
}

func TestGetAllLocalConnections(t *testing.T) {

	var testReceptors = map[string]map[string]Receptor{
		"0000001": {"node-a": &MockReceptor{},
			"node-b": &MockReceptor{}},
		"0000002": {"node-a": &MockReceptor{}},
		"0000003": {"node-a": &MockReceptor{}},
	}
	cm := NewLocalConnectionManager()
	for account, receptorMap := range testReceptors {
		for nodeID, receptor := range receptorMap {
			cm.Register(context.TODO(), account, nodeID, receptor)
		}
	}

	receptorMap := cm.GetAllConnections(context.TODO())

	if cmp.Equal(testReceptors, receptorMap) != true {
		t.Fatalf("Excepted receptor map and actual receptor map do not match.  Excpected %+v, Actual %+v",
			testReceptors, receptorMap)
	}
}

func TestGetAllLocalConnectionsWithNoRegisteredReceptors(t *testing.T) {
	cm := NewLocalConnectionManager()
	receptorMap := cm.GetAllConnections(context.TODO())
	if len(receptorMap) != 0 {
		t.Fatalf("Expected to find 0 connections, but found %d connections", len(receptorMap))
	}
}

func TestRegisterLocalConnectionsWithDuplicateNodeIDs(t *testing.T) {
	accountNumber := "123"
	nodeID := "456"
	expectedReceptorObj := new(MockReceptor)
	secondReceptorObj := new(MockReceptor)

	// This seems silly but it tripped me up since pointers to empty structs have the same value
	if expectedReceptorObj == secondReceptorObj {
		t.Fatalf("Expected the test receptor objects to be different")
	}

	cm := NewLocalConnectionManager()

	err := cm.Register(context.TODO(), accountNumber, nodeID, expectedReceptorObj)
	if err != nil {
		t.Fatalf("Expected the error to be nil")
	}

	err = cm.Register(context.TODO(), accountNumber, nodeID, secondReceptorObj)
	if err == nil {
		t.Fatalf("Expected an error instance to be returned in the case of duplicate registration")
	}

	actualReceptorObj := cm.GetConnection(context.TODO(), accountNumber, nodeID)

	if actualReceptorObj != expectedReceptorObj {
		t.Fatalf("Expected to find the connection that was registered first")
	}
}
