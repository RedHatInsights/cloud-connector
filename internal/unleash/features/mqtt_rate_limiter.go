package features

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/unleash"
	"github.com/sirupsen/logrus"
)

const (
	// MQTTRateLimiterFeatureFlag is the Unleash feature flag for MQTT rate limiting
	MQTTRateLimiterFeatureFlag = "cloud-connector-mqtt-rate-limiter"

	// Variant names matching Unleash dashboard configuration
	//
	// VariantDropMessage: When rate threshold exceeded, drop ALL messages immediately
	VariantDropMessage = "drop_message"
	// VariantTestOnly: Track metrics and log threshold crossings, but accept all messages (monitoring only, no enforcement)
	VariantTestOnly = "test_only"
)

// RateLimiterState holds the runtime state for the MQTT rate limiter
// All fields are determined by precedence: Defaults → Env → Unleash
type RateLimiterState struct {
	Enabled   bool
	Variant   string
	Threshold int
	Window    time.Duration
}

// VariantPayload represents the optional JSON payload in Unleash variant
type VariantPayload struct {
	Threshold *int    `json:"threshold,omitempty"`
	Window    *string `json:"window,omitempty"`
}

// GetMQTTRateLimiterState determines the current MQTT rate limiter state
// Configuration priority:
//  1. Defaults (from code)
//  2. Environment variables
//  3. Unleash feature flag variant (overrides env vars)
//  4. Unleash variant payload (overrides threshold/window/staleThreshold)
//
// Returns the effective state
func GetMQTTRateLimiterState(cfg *config.Config, log *logrus.Entry) RateLimiterState {
	// Start with environment variable / defaults
	state := RateLimiterState{
		Enabled:   cfg.MqttRateLimiterEnabled,
		Variant:   cfg.MqttRateLimiterVariant,
		Threshold: cfg.MqttRateLimiterThreshold,
		Window:    cfg.MqttRateLimiterWindow,
	}

	// Check circuit breaker: skip Unleash if disabled for this feature
	if cfg.MqttRateLimiterUnleashDisabled {
		log.WithFields(logrus.Fields{
			"source":    "environment",
			"enabled":   state.Enabled,
			"variant":   state.Variant,
			"threshold": state.Threshold,
			"window":    state.Window,
			"reason":    "unleash_disabled_for_mqtt_rate_limiter",
		}).Debug("MQTT rate limiter Unleash check disabled, using environment config only")
		return state
	}

	// If Unleash is enabled globally AND successfully initialized, check feature flag
	if cfg.UnleashEnabled && unleash.IsInitialized() {
		variant := unleash.GetVariant(MQTTRateLimiterFeatureFlag)

		// Check if variant is enabled with a supported variant name
		if variant != nil && variant.Enabled {
			// Only enable if variant name is supported
			if IsValidVariant(variant.Name) {
				state.Enabled = true
				state.Variant = variant.Name

				// Parse payload if present (only for supported variants)
				if variant.Payload.Type == "json" && variant.Payload.Value != "" {
					payload := ParseVariantPayload(variant.Payload.Value, log)
					if !ValidateVariantPayload(&state, payload, log) {
						log.Debug("One or more variant fields were invalid and deferred to config")
					}
				}

				log.WithFields(logrus.Fields{
					"source":    "unleash",
					"feature":   MQTTRateLimiterFeatureFlag,
					"variant":   state.Variant,
					"enabled":   true,
					"threshold": state.Threshold,
					"window":    state.Window,
				}).Debug("MQTT rate limiter configuration from Unleash")
			} else {
				// Unknown variant: keep environment config and log warning
				log.WithFields(logrus.Fields{
					"feature":      MQTTRateLimiterFeatureFlag,
					"variant_name": variant.Name,
					"supported":    fmt.Sprintf("%s, %s", VariantDropMessage, VariantTestOnly),
				}).Warn("Unsupported Unleash variant for MQTT rate limiter, using environment config")
			}
		} else if variant != nil {
			// Feature flag is disabled in Unleash, override to disabled
			state.Enabled = false

			log.WithFields(logrus.Fields{
				"source":  "unleash",
				"feature": MQTTRateLimiterFeatureFlag,
				"enabled": false,
			}).Debug("MQTT rate limiter disabled by Unleash")
		}
		// If variant == nil (Unleash error), use env var (already set above)
	} else {
		log.WithFields(logrus.Fields{
			"source":    "environment",
			"enabled":   state.Enabled,
			"variant":   state.Variant,
			"threshold": state.Threshold,
			"window":    state.Window,
		}).Debug("MQTT rate limiter configuration from environment")
	}

	return state
}

// ParseVariantPayload parses the JSON payload from Unleash variant
// Returns nil if parsing fails
func ParseVariantPayload(jsonStr string, log *logrus.Entry) *VariantPayload {
	var payload VariantPayload
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		log.WithError(err).WithField("payload", jsonStr).Warn("Failed to parse Unleash variant payload")
		return nil
	}
	return &payload
}

// ValidateVariantPayload validates and applies Unleash variant payload to state
// Returns true if all provided fields were valid and applied
// Returns false if any field was invalid
func ValidateVariantPayload(state *RateLimiterState, payload *VariantPayload, log *logrus.Entry) bool {
	if payload == nil {
		log.Debug("No payload. Using config variables")
		return true
	}

	allValid := true

	if payload.Threshold != nil {
		if *payload.Threshold > 0 {
			state.Threshold = *payload.Threshold
		} else {
			allValid = false
			log.WithField("threshold", *payload.Threshold).Warn("Invalid threshold from Unleash payload (must be > 0), ignoring")
		}
	}

	if payload.Window != nil {
		if duration, err := time.ParseDuration(*payload.Window); err == nil {
			if duration > 0 {
				state.Window = duration
			} else {
				allValid = false
				log.WithField("window", *payload.Window).Warn("Invalid window from Unleash payload (must be > 0), ignoring")
			}
		} else {
			allValid = false
			log.WithError(err).WithField("window", *payload.Window).Warn("Failed to parse window from Unleash payload")
		}
	}

	if allValid {
		return true
	}

	return false
}

// IsValidVariant checks if a variant string is valid
func IsValidVariant(variant string) bool {
	return variant == VariantDropMessage || variant == VariantTestOnly
}
