package mqtt

import (
	"testing"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/unleash/features"
)

func init() {
	logger.InitLogger()
}

func TestTrackMessage_WindowReset(t *testing.T) {
	cfg := &config.Config{
		MqttRateLimiterThreshold: 50,
		MqttRateLimiterWindow:    10 * time.Second,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	clientID := "test-client"
	threshold := 50
	window := 10 * time.Second

	// First window: send 51 messages
	now := time.Now()
	for i := 0; i < 51; i++ {
		count, wasOver := rl.trackMessage(clientID, now, threshold, window)
		if i < 50 {
			if count != i+1 {
				t.Errorf("Message %d: expected count=%d, got %d", i, i+1, count)
			}
			if wasOver {
				t.Errorf("Message %d: expected wasOver=false, got true", i)
			}
		} else {
			// Message 51 (i=50)
			if count != 51 {
				t.Errorf("Message 51: expected count=51, got %d", count)
			}
			if wasOver {
				t.Errorf("Message 51: expected wasOver=false (first crossing), got true")
			}
		}
	}

	// Verify over-threshold state is set
	_, wasOver := rl.trackMessage(clientID, now, threshold, window)
	if !wasOver {
		t.Error("Expected wasOver=true after threshold crossed")
	}

	// Window expires (11 seconds later)
	futureTime := now.Add(11 * time.Second)
	count, wasOver := rl.trackMessage(clientID, futureTime, threshold, window)

	// CRITICAL: Window reset should clear over-threshold state
	if count != 1 {
		t.Errorf("After window reset: expected count=1, got %d", count)
	}
	if wasOver {
		t.Errorf("After window reset: expected wasOver=false (new window), got true")
	}
}

func TestTrackMessage_AtThreshold(t *testing.T) {
	cfg := &config.Config{
		MqttRateLimiterThreshold: 50,
		MqttRateLimiterWindow:    10 * time.Second,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	clientID := "test-client"
	threshold := 50
	window := 10 * time.Second
	now := time.Now()

	// Send exactly 50 messages
	for i := 0; i < 50; i++ {
		rl.trackMessage(clientID, now, threshold, window)
	}

	count, wasOver := rl.trackMessage(clientID, now, threshold, window)

	// At threshold should NOT be over
	if count != 51 {
		t.Errorf("Expected count=51, got %d", count)
	}
	if wasOver {
		t.Error("At message 51, wasOver should still be false (first time over)")
	}
}

func TestShouldAccept_BelowThreshold(t *testing.T) {
	cfg := &config.Config{
		MqttRateLimiterEnabled:   true,
		MqttRateLimiterThreshold: 50,
		MqttRateLimiterWindow:    10 * time.Second,
		MqttRateLimiterVariant:   "drop_message",
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()
	log := logger.Log.WithField("test", "shouldAccept")

	// Mock state (simulating getState() return)
	// We need to test ShouldAcceptMessage directly, but it calls getState()
	// For now, test via trackMessage logic which is already tested above

	clientID := "test-client"
	payload := []byte(`{"sent":"2024-01-01T00:00:00Z"}`)

	// Below threshold should accept
	accepted := rl.ShouldAcceptMessage(clientID, payload, log)
	if !accepted {
		t.Error("Expected message to be accepted when below threshold")
	}
}

func TestShouldAccept_DropMessageVariant(t *testing.T) {
	cfg := &config.Config{
		MqttRateLimiterEnabled:   true,
		MqttRateLimiterThreshold: 2, // Very low threshold for testing
		MqttRateLimiterWindow:    10 * time.Second,
		MqttRateLimiterVariant:   "drop_message",
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()
	log := logger.Log.WithField("test", "shouldAccept")

	clientID := "test-client"
	now := time.Now()
	payload := []byte(`{"sent":"` + now.Format(time.RFC3339Nano) + `"}`)

	// Message 1-2: accepted
	rl.ShouldAcceptMessage(clientID, payload, log)
	rl.ShouldAcceptMessage(clientID, payload, log)

	// Message 3: over threshold with drop_message variant
	accepted := rl.ShouldAcceptMessage(clientID, payload, log)
	if accepted {
		t.Error("Expected drop_message variant to drop ALL messages when over threshold")
	}
}

func TestShouldAccept_TestOnlyVariant(t *testing.T) {
	cfg := &config.Config{
		MqttRateLimiterEnabled:   true,
		MqttRateLimiterThreshold: 2, // Very low threshold for testing
		MqttRateLimiterWindow:    10 * time.Second,
		MqttRateLimiterVariant:   "test_only",
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()
	log := logger.Log.WithField("test", "shouldAccept")

	clientID := "test-client"
	now := time.Now()
	payload := []byte(`{"sent":"` + now.Format(time.RFC3339Nano) + `"}`)

	// Message 1-2: accepted (below threshold)
	if !rl.ShouldAcceptMessage(clientID, payload, log) {
		t.Error("Expected test_only variant to accept messages below threshold")
	}
	if !rl.ShouldAcceptMessage(clientID, payload, log) {
		t.Error("Expected test_only variant to accept messages at threshold")
	}

	// Message 3+: over threshold with test_only variant
	// Should still accept (monitoring only, no enforcement)
	for i := 3; i <= 10; i++ {
		accepted := rl.ShouldAcceptMessage(clientID, payload, log)
		if !accepted {
			t.Errorf("Message %d: Expected test_only variant to accept ALL messages even over threshold", i)
		}
	}
}

func TestShouldAccept_Disabled(t *testing.T) {
	cfg := &config.Config{
		MqttRateLimiterEnabled:   false,
		MqttRateLimiterThreshold: 1, // Very low
		MqttRateLimiterWindow:    10 * time.Second,
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()
	log := logger.Log.WithField("test", "shouldAccept")

	clientID := "test-client"
	payload := []byte(`{"sent":"2024-01-01T00:00:00Z"}`)

	// Send many messages - all should be accepted when disabled
	for i := 0; i < 100; i++ {
		accepted := rl.ShouldAcceptMessage(clientID, payload, log)
		if !accepted {
			t.Errorf("Message %d: Expected acceptance when rate limiter disabled", i)
		}
	}
}

func TestGetState_Caching(t *testing.T) {
	cfg := &config.Config{
		MqttRateLimiterEnabled:   true,
		MqttRateLimiterThreshold: 50,
		MqttRateLimiterWindow:    10 * time.Second,
		MqttRateLimiterVariant:   "drop_message",
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	// State should be initialized immediately
	state1 := rl.getState()
	time1 := rl.lastStateTime

	// Multiple rapid calls should return same cached state
	for i := 0; i < 10; i++ {
		state := rl.getState()
		if state.Enabled != state1.Enabled || state.Threshold != state1.Threshold {
			t.Errorf("Call %d: Expected identical state from cache", i)
		}
	}

	// lastStateTime should be unchanged (no refresh on read)
	time2 := rl.lastStateTime
	if time1 != time2 {
		t.Error("Expected lastStateTime unchanged (background goroutine handles refresh)")
	}
}

func TestConcurrentAccess(t *testing.T) {
	cfg := &config.Config{
		MqttRateLimiterEnabled:   true,
		MqttRateLimiterThreshold: 100,
		MqttRateLimiterWindow:    10 * time.Second,
		MqttRateLimiterVariant:   "drop_message",
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()
	log := logger.Log.WithField("test", "concurrent")

	clientID := "test-client"
	payload := []byte(`{"sent":"2024-01-01T00:00:00Z"}`)

	// Simulate concurrent messages from same client
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				rl.ShouldAcceptMessage(clientID, payload, log)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final count is exactly 100 (10 goroutines * 10 messages)
	// Get one more message to check count
	threshold := 100
	window := 10 * time.Second
	count, _ := rl.trackMessage(clientID, time.Now(), threshold, window)

	if count != 101 {
		t.Errorf("Expected count=101 after concurrent access, got %d", count)
	}
}

func TestValidateConfig_InvalidValues(t *testing.T) {
	testCases := []struct {
		name              string
		envVars           map[string]string
		expectedThreshold int
		expectedWindow    time.Duration
		expectedVariant   string
	}{
		{
			name: "Invalid threshold (zero)",
			envVars: map[string]string{
				"CLOUD_CONNECTOR_MQTT_RATE_LIMITER_THRESHOLD": "0",
			},
			expectedThreshold: 50, // Default
			expectedWindow:    10 * time.Second,
			expectedVariant:   "test_only",
		},
		{
			name: "Invalid threshold (negative)",
			envVars: map[string]string{
				"CLOUD_CONNECTOR_MQTT_RATE_LIMITER_THRESHOLD": "-10",
			},
			expectedThreshold: 50, // Default
			expectedWindow:    10 * time.Second,
			expectedVariant:   "test_only",
		},
		{
			name: "Invalid window (zero)",
			envVars: map[string]string{
				"CLOUD_CONNECTOR_MQTT_RATE_LIMITER_WINDOW": "0s",
			},
			expectedThreshold: 50,
			expectedWindow:    10 * time.Second, // Default
			expectedVariant:   "test_only",
		},
		{
			name: "Invalid window (negative)",
			envVars: map[string]string{
				"CLOUD_CONNECTOR_MQTT_RATE_LIMITER_WINDOW": "-5s",
			},
			expectedThreshold: 50,
			expectedWindow:    10 * time.Second, // Default
			expectedVariant:   "test_only",
		},
		{
			name: "Invalid variant",
			envVars: map[string]string{
				"CLOUD_CONNECTOR_MQTT_RATE_LIMITER_VARIANT": "unknown_variant",
			},
			expectedThreshold: 50,
			expectedWindow:    10 * time.Second,
			expectedVariant:   "test_only", // Default
		},
		{
			name: "Multiple invalid values",
			envVars: map[string]string{
				"CLOUD_CONNECTOR_MQTT_RATE_LIMITER_THRESHOLD": "-1",
				"CLOUD_CONNECTOR_MQTT_RATE_LIMITER_WINDOW":    "0s",
				"CLOUD_CONNECTOR_MQTT_RATE_LIMITER_VARIANT":   "bad",
			},
			expectedThreshold: 50,               // Default
			expectedWindow:    10 * time.Second, // Default
			expectedVariant:   "test_only",      // Default
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			// Call the real GetConfig function
			cfg := config.GetConfig()

			// Verify defaults were applied
			if cfg.MqttRateLimiterThreshold != tc.expectedThreshold {
				t.Errorf("Expected threshold=%d, got %d", tc.expectedThreshold, cfg.MqttRateLimiterThreshold)
			}
			if cfg.MqttRateLimiterWindow != tc.expectedWindow {
				t.Errorf("Expected window=%v, got %v", tc.expectedWindow, cfg.MqttRateLimiterWindow)
			}
			if cfg.MqttRateLimiterVariant != tc.expectedVariant {
				t.Errorf("Expected variant=%s, got %s", tc.expectedVariant, cfg.MqttRateLimiterVariant)
			}
		})
	}
}

func TestIncidentMetricOnlyOnFirstCrossing(t *testing.T) {
	cfg := &config.Config{
		MqttRateLimiterEnabled:   true,
		MqttRateLimiterThreshold: 2,
		MqttRateLimiterWindow:    10 * time.Second,
		MqttRateLimiterVariant:   "drop_message",
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	clientID := "test-client"
	threshold := 2
	window := 10 * time.Second
	now := time.Now()

	// Message 1-2: below threshold
	_, wasOver := rl.trackMessage(clientID, now, threshold, window)
	if wasOver {
		t.Error("Message 1: should not be over threshold")
	}
	_, wasOver = rl.trackMessage(clientID, now, threshold, window)
	if wasOver {
		t.Error("Message 2: should not be over threshold")
	}

	// Message 3: FIRST crossing
	_, wasOver = rl.trackMessage(clientID, now, threshold, window)
	if wasOver {
		t.Error("Message 3: should be first crossing (wasOver=false)")
	}

	// Message 4: subsequent crossing
	_, wasOver = rl.trackMessage(clientID, now, threshold, window)
	if !wasOver {
		t.Error("Message 4: should be continuation (wasOver=true)")
	}
}

func TestUnknownVariant(t *testing.T) {
	cfg := &config.Config{
		MqttRateLimiterEnabled:   true,
		MqttRateLimiterThreshold: 1,
		MqttRateLimiterWindow:    10 * time.Second,
		MqttRateLimiterVariant:   "unknown_variant",
	}
	rl := NewRateLimiter(cfg)
	defer rl.Stop()
	log := logger.Log.WithField("test", "unknownVariant")

	clientID := "test-client"
	payload := []byte(`{"sent":"2024-01-01T00:00:00Z"}`)

	// Get over threshold
	rl.ShouldAcceptMessage(clientID, payload, log)

	// Message 2: over threshold with unknown variant
	// Should accept (safe default)
	accepted := rl.ShouldAcceptMessage(clientID, payload, log)
	if !accepted {
		t.Error("Expected unknown variant to accept messages (safe default)")
	}
}

func TestIsValidVariant(t *testing.T) {
	testCases := []struct {
		variant string
		valid   bool
	}{
		{"drop_message", true},
		{"test_only", true},
		{"typo", false},
		{"", false},
		{"DROP_MESSAGE", false}, // Case sensitive
		{"TEST_ONLY", false},    // Case sensitive
	}

	for _, tc := range testCases {
		t.Run(tc.variant, func(t *testing.T) {
			result := features.IsValidVariant(tc.variant)
			if result != tc.valid {
				t.Errorf("IsValidVariant(%q) = %v, expected %v", tc.variant, result, tc.valid)
			}
		})
	}
}
