package mqtt

import (
	"sync"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"
	"github.com/RedHatInsights/cloud-connector/internal/unleash/features"
	cache "github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
)

// clientRate tracks the message count, window start time, and threshold state for a client
type clientRate struct {
	count           int
	windowStart     time.Time
	isOverThreshold bool
}

// RateLimiter tracks message rates per client and enforces rate limits
type RateLimiter struct {
	// Cache for client_id -> clientRate (count, window start, threshold state)
	// Expires after 5 minutes of inactivity, cleanup runs every 1 minute
	clientRates *cache.Cache

	// Mutex to protect get-modify-set operations in trackMessage
	// Ensures atomic updates to client rate counts
	mu sync.Mutex

	// Configuration (static, from config file / env vars)
	cfg *config.Config

	// Cached Unleash state to avoid per-message overhead
	// Refreshed by background goroutine every 15s
	lastState     features.RateLimiterState
	lastStateTime time.Time
	stateMu       sync.RWMutex

	// Background goroutine lifecycle management
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewRateLimiter creates a new rate limiter instance
func NewRateLimiter(cfg *config.Config) *RateLimiter {
	rl := &RateLimiter{
		clientRates: cache.New(5*time.Minute, 1*time.Minute),
		cfg:         cfg,
		stopCh:      make(chan struct{}),
	}

	// Initialize state immediately (before starting goroutine)
	log := logger.Log.WithField("component", "rate_limiter")
	rl.lastState = features.GetMQTTRateLimiterState(cfg, log)
	rl.lastStateTime = time.Now()

	// Start background refresh goroutine
	rl.wg.Add(1)
	go rl.stateRefreshLoop()

	return rl
}

// getState returns the current rate limiter state from cache
// State is refreshed by background goroutine every 15 seconds
// This is a fast read-only operation on the hot path
func (rl *RateLimiter) getState() features.RateLimiterState {
	rl.stateMu.RLock()
	defer rl.stateMu.RUnlock()
	return rl.lastState
}

// refreshState updates the cached Unleash state
// Called by background goroutine every 15 seconds
func (rl *RateLimiter) refreshState() {
	log := logger.Log.WithField("component", "rate_limiter")
	newState := features.GetMQTTRateLimiterState(rl.cfg, log)

	rl.stateMu.Lock()
	rl.lastState = newState
	rl.lastStateTime = time.Now()
	rl.stateMu.Unlock()

	log.WithFields(logrus.Fields{
		"enabled":   newState.Enabled,
		"variant":   newState.Variant,
		"threshold": newState.Threshold,
		"window":    newState.Window,
	}).Debug("Rate limiter state refreshed")
}

// stateRefreshLoop runs in background, refreshing Unleash state every 15 seconds
// Keeps Unleash calls out of the message processing hot path
func (rl *RateLimiter) stateRefreshLoop() {
	defer rl.wg.Done()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.refreshState()
		case <-rl.stopCh:
			return
		}
	}
}

// ShouldAcceptMessage determines if a message should be accepted or dropped based on rate and variant
// Returns true to accept (write to Kafka), false to drop
func (rl *RateLimiter) ShouldAcceptMessage(clientID string, payload []byte, log *logrus.Entry) bool {
	// Get cached state (refreshed by background goroutine every 15s)
	state := rl.getState()

	// Check if rate limiting is enabled
	if !state.Enabled {
		return true
	}

	now := time.Now()

	// Track this message and get current rate
	messageCount, wasOverThreshold := rl.trackMessage(clientID, now, state.Threshold, state.Window)

	// Check if rate exceeds threshold
	if messageCount <= state.Threshold {
		// Rate is normal, accept message
		return true
	}

	// Rate exceeded threshold
	// Only increment incident counter if this is a NEW crossing (wasn't over threshold before)
	if !wasOverThreshold {
		log.WithFields(logrus.Fields{
			"client_id":     clientID,
			"message_count": messageCount,
			"threshold":     state.Threshold,
			"window":        state.Window,
			"variant":       state.Variant,
		}).Warn("Client crossed rate threshold")

		metrics.rateLimiterRateExceeded.WithLabelValues(state.Variant).Inc()
	}

	// Handle based on variant:
	//   drop_message: Drop ALL messages when rate exceeded
	// Variant: drop_message
	// When rate exceeded, drop ALL messages immediately
	// Variant: test_only
	// Just log that we triggered a rate limiting action
	switch state.Variant {
	case features.VariantDropMessage:
		log.WithFields(logrus.Fields{
			"client_id": clientID,
			"variant":   state.Variant,
			"action":    "dropped",
		}).Debug("Rate limit exceeded")

		metrics.rateLimiterMessagesDropped.WithLabelValues("drop_message").Inc()
		return false
	case features.VariantTestOnly:
		log.WithFields(logrus.Fields{
			"client_id": clientID,
			"variant":   state.Variant,
			"action":    "testing",
		}).Debug("Rate limit exceeded")

		metrics.rateLimiterMessagesAcceptedOverRate.WithLabelValues(state.Variant).Inc()
		return true
	default:
		log.WithFields(logrus.Fields{
			"client_id": clientID,
			"variant":   state.Variant,
			"action":    "unknown variant",
		}).Debug("Rate limit exceeded")

		return true
	}
}

// trackMessage tracks the current message and returns the count in the current window
// Returns: (message count in window, was client already over threshold before this message)
// Uses threshold/window from parameters (which may be overridden by Unleash)
func (rl *RateLimiter) trackMessage(clientID string, now time.Time, threshold int, window time.Duration) (int, bool) {
	// Lock to ensure atomic get-modify-set operation
	// Prevents race condition where concurrent messages can both read same count,
	// both increment, and one update is lost
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Get current rate from cache (defaults to zero values if not found)
	var rate clientRate
	if cached, found := rl.clientRates.Get(clientID); found {
		rate = cached.(clientRate)
	}

	// Remember if client was already over threshold before this message
	alreadyPassedThreshold := rate.isOverThreshold

	// Check if window has expired - reset to new window
	if now.Sub(rate.windowStart) >= window {
		rate = clientRate{
			count:           0,
			windowStart:     now,
			isOverThreshold: false,
		}
		alreadyPassedThreshold = false
	}

	// Increment count for this message
	rate.count++

	// Update threshold state based on new count
	rate.isOverThreshold = rate.count > threshold

	// Store updated state back in cache
	rl.clientRates.Set(clientID, rate, cache.DefaultExpiration)

	return rate.count, alreadyPassedThreshold
}

// Stop gracefully shuts down the rate limiter's background goroutine
// Should be called during application shutdown
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
	rl.wg.Wait()
}
