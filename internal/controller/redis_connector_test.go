package controller

import (
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/platform/utils"
	"github.com/go-playground/assert/v2"

	"github.com/alicebob/miniredis"
	"github.com/go-redis/redis"
)

var testHost string = utils.GetHostname()

func newTestRedisClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})
}

func TestRegisterWithRedis(t *testing.T) {
	s, _ := miniredis.Run()
	defer s.Close()

	c := newTestRedisClient(s.Addr())

	tests := []struct {
		account   string
		nodeID    string
		hostname  string
		accIndex  []string
		connIndex []string
		podIndex  []string
		err       error
	}{
		{
			account:   "01",
			nodeID:    "node-a",
			hostname:  testHost,
			accIndex:  []string{"node-a:" + testHost},
			connIndex: []string{"01:node-a:" + testHost},
			podIndex:  []string{"01:node-a"},
			err:       nil,
		},
		{
			account:   "01",
			nodeID:    "node-b",
			hostname:  testHost,
			accIndex:  []string{"node-a:" + testHost, "node-b:" + testHost},
			connIndex: []string{"01:node-a:" + testHost, "01:node-b:" + testHost},
			podIndex:  []string{"01:node-a", "01:node-b"},
			err:       nil,
		},
		{
			account:   "02",
			nodeID:    "node-a",
			hostname:  testHost,
			accIndex:  []string{"node-a:" + testHost},
			connIndex: []string{"01:node-a:" + testHost, "01:node-b:" + testHost, "02:node-a:" + testHost},
			podIndex:  []string{"01:node-a", "01:node-b", "02:node-a"},
			err:       nil,
		},
	}

	for _, tc := range tests {
		got := RegisterWithRedis(c, tc.account, tc.nodeID, tc.hostname)
		if got != tc.err {
			t.Fatalf("expected: %v, got: %v", tc.err, got)
		}
		assert.Equal(t, c.Get(tc.account+":"+tc.nodeID).Val(), tc.hostname)
		assert.Equal(t, c.SMembers(tc.account).Val(), tc.accIndex)
		assert.Equal(t, c.SMembers("connections").Val(), tc.connIndex)
		assert.Equal(t, c.SMembers(tc.hostname).Val(), tc.podIndex)
	}
}

func TestDuplicateRegisterWithRedis(t *testing.T) {
	s, _ := miniredis.Run()
	defer s.Close()

	c := newTestRedisClient(s.Addr())

	tests := []struct {
		account          string
		nodeID           string
		hostname         string
		expectedHost     string
		accIndex         []string
		connIndex        []string
		podIndex         []string
		expectedPodIndex []string
		err              error
	}{
		{
			account:          "01",
			nodeID:           "node-a",
			hostname:         testHost,
			expectedHost:     testHost,
			accIndex:         []string{"node-a:" + testHost},
			connIndex:        []string{"01:node-a:" + testHost},
			podIndex:         []string{"01:node-a"},
			expectedPodIndex: []string{"01:node-a"},
			err:              nil,
		},
		{
			account:          "01",
			nodeID:           "node-a",
			hostname:         "dupe-conn",
			expectedHost:     testHost,
			accIndex:         []string{"node-a:" + testHost},
			connIndex:        []string{"01:node-a:" + testHost},
			podIndex:         []string{},
			expectedPodIndex: []string{"01:node-a"},
			err:              DuplicateConnectionError{},
		},
		{
			account:          "02",
			nodeID:           "node-a",
			hostname:         testHost,
			expectedHost:     testHost,
			accIndex:         []string{"node-a:" + testHost},
			connIndex:        []string{"01:node-a:" + testHost, "02:node-a:" + testHost},
			podIndex:         []string{"01:node-a", "02:node-a"},
			expectedPodIndex: []string{"01:node-a", "02:node-a"},
			err:              nil,
		},
		{
			account:          "02",
			nodeID:           "node-a",
			hostname:         "dupe-conn",
			expectedHost:     testHost,
			accIndex:         []string{"node-a:" + testHost},
			connIndex:        []string{"01:node-a:" + testHost, "02:node-a:" + testHost},
			podIndex:         []string{},
			expectedPodIndex: []string{"01:node-a", "02:node-a"},
			err:              DuplicateConnectionError{},
		},
	}

	for _, tc := range tests {
		got := RegisterWithRedis(c, tc.account, tc.nodeID, tc.hostname)
		if got != tc.err {
			t.Fatalf("expected: %v, got: %v", tc.err, got)
		}
		assert.Equal(t, c.Get(tc.account+":"+tc.nodeID).Val(), tc.expectedHost)
		assert.Equal(t, c.SMembers(tc.account).Val(), tc.accIndex)
		assert.Equal(t, c.SMembers("connections").Val(), tc.connIndex)
		assert.Equal(t, c.SMembers(tc.hostname).Val(), tc.podIndex)
		assert.Equal(t, c.SMembers(tc.expectedHost).Val(), tc.expectedPodIndex)
	}
}

func TestUnregisterWithRedis(t *testing.T) {
	s, _ := miniredis.Run()
	defer s.Close()

	c := newTestRedisClient(s.Addr())

	_ = RegisterWithRedis(c, "01", "node-a", testHost)
	_ = RegisterWithRedis(c, "01", "node-b", testHost)
	_ = RegisterWithRedis(c, "01", "node-c", testHost)
	_ = RegisterWithRedis(c, "01", "node-d", testHost)

	tests := []struct {
		account   string
		nodeID    string
		remaining []string
	}{
		{account: "01", nodeID: "node-a", remaining: []string{"01:node-b", "01:node-c", "01:node-d"}},
		{account: "01", nodeID: "node-not-found", remaining: []string{"01:node-b", "01:node-c", "01:node-d"}},
		{account: "01", nodeID: "node-b", remaining: []string{"01:node-c", "01:node-d"}},
		{account: "01", nodeID: "node-c", remaining: []string{"01:node-d"}},
	}

	for _, tc := range tests {
		UnregisterWithRedis(c, tc.account, tc.nodeID, testHost)
		assert.Equal(t, c.Get(tc.account+":"+tc.nodeID).Err(), redis.Nil)
		assert.Equal(t, c.SMembers(testHost).Val(), tc.remaining)
	}

	assert.Equal(t, c.SMembers("01").Val(), []string{"node-d:" + testHost})
	assert.Equal(t, c.SMembers("connections").Val(), []string{"01:node-d:" + testHost})
	assert.Equal(t, c.SMembers(testHost).Val(), []string{"01:node-d"})
	assert.Equal(t, c.Get("01:node-d").Val(), testHost)
}

func TestGetRedisConnection(t *testing.T) {
	s, _ := miniredis.Run()
	defer s.Close()

	c := newTestRedisClient(s.Addr())

	_ = RegisterWithRedis(c, "01", "node-a", testHost)

	tests := []struct {
		account string
		nodeID  string
		want    string
		err     error
	}{
		{account: "01", nodeID: "node-a", want: testHost, err: nil},
		{account: "01", nodeID: "bad-node", want: "", err: redis.Nil},
	}

	for _, tc := range tests {
		conn, err := GetRedisConnection(c, tc.account, tc.nodeID)
		assert.Equal(t, conn, tc.want)
		assert.Equal(t, err, tc.err)
	}
}

func TestGetRedisConnectionsByAccount(t *testing.T) {
	s, _ := miniredis.Run()
	defer s.Close()

	c := newTestRedisClient(s.Addr())

	_ = RegisterWithRedis(c, "01", "node-a", testHost)
	_ = RegisterWithRedis(c, "01", "node-b", testHost)
	_ = RegisterWithRedis(c, "02", "node-c", testHost)

	tests := []struct {
		account string
		want    map[string]string
		err     error
	}{
		{account: "01", want: map[string]string{"node-a": testHost, "node-b": testHost}, err: nil},
		{account: "02", want: map[string]string{"node-c": testHost}, err: nil},
		{account: "not-found", want: map[string]string{}, err: nil},
	}

	for _, tc := range tests {
		res, err := GetRedisConnectionsByAccount(c, tc.account)
		assert.Equal(t, res, tc.want)
		assert.Equal(t, err, tc.err)
	}
}

func TestGetRedisConnectionsByHost(t *testing.T) {
	s, _ := miniredis.Run()
	defer s.Close()

	c := newTestRedisClient(s.Addr())

	_ = RegisterWithRedis(c, "01", "node-a", testHost)
	_ = RegisterWithRedis(c, "01", "node-b", testHost)
	_ = RegisterWithRedis(c, "02", "node-b", testHost)
	_ = RegisterWithRedis(c, "03", "node-c", "gateway-pod-9")

	tests := []struct {
		hostname string
		want     map[string][]string
		err      error
	}{
		{hostname: testHost, want: map[string][]string{"01": {"node-a", "node-b"}, "02": {"node-b"}}, err: nil},
		{hostname: "gateway-pod-9", want: map[string][]string{"03": {"node-c"}}, err: nil},
		{hostname: "not-found", want: map[string][]string{}, err: nil},
	}

	for _, tc := range tests {
		res, err := GetRedisConnectionsByHost(c, tc.hostname)
		assert.Equal(t, res, tc.want)
		assert.Equal(t, err, tc.err)
	}
}

func TestGetAllRedisConnections(t *testing.T) {
	s, _ := miniredis.Run()
	defer s.Close()

	c := newTestRedisClient(s.Addr())

	_ = RegisterWithRedis(c, "01", "node-a", testHost)
	_ = RegisterWithRedis(c, "01", "node-b", testHost)
	_ = RegisterWithRedis(c, "02", "node-b", testHost)

	res, err := GetAllRedisConnections(c)
	if err != nil {
		t.Fatalf("error getting all connections: %v", err)
	}

	assert.Equal(t, map[string]map[string]string{
		"01": {"node-a": testHost, "node-b": testHost},
		"02": {"node-b": testHost},
	}, res)
}
