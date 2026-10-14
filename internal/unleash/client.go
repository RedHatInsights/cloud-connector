package unleash

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/Unleash/unleash-go-sdk/v6"
	"github.com/Unleash/unleash-go-sdk/v6/api"
	ucontext "github.com/Unleash/unleash-go-sdk/v6/context"
	"github.com/sirupsen/logrus"
)

var (
	// Track whether Unleash successfully initialized
	initialized bool
	initMu      sync.RWMutex
)

// Initialize initializes the Unleash client
// Returns error if initialization fails, but this is non-fatal - the application can continue
// If Unleash is unavailable, feature flags will fall back to environment variables
func Initialize(cfg *config.Config, log *logrus.Entry) error {
	// Check if Unleash is enabled
	if !cfg.UnleashEnabled {
		log.Info("Unleash feature flags disabled")
		return nil
	}

	// Validate required configuration
	url := cfg.UnleashURL
	if url == "" {
		return fmt.Errorf("CLOUD_CONNECTOR_UNLEASH_URL is required when CLOUD_CONNECTOR_UNLEASH_ENABLED=true")
	}

	apiToken := cfg.UnleashAPIToken
	if apiToken == "" {
		return fmt.Errorf("CLOUD_CONNECTOR_UNLEASH_API_TOKEN is required when CLOUD_CONNECTOR_UNLEASH_ENABLED=true")
	}

	appName := cfg.UnleashAppName
	environment := cfg.UnleashEnvironment

	log.WithFields(logrus.Fields{
		"url":         url,
		"app_name":    appName,
		"environment": environment,
	}).Info("Initializing Unleash client")

	// Initialize Unleash client
	err := unleash.Initialize(
		// Event listener for logging
		unleash.WithListener(NewListener(log)),

		// Application identification
		unleash.WithAppName(appName),
		unleash.WithUrl(url),
		unleash.WithEnvironment(environment),

		// Polling intervals
		unleash.WithRefreshInterval(15*time.Second), // Poll for feature flag updates every 15s
		unleash.WithMetricsInterval(60*time.Second), // Send usage metrics every 60s

		// Authentication
		unleash.WithCustomHeaders(http.Header{
			"Authorization": {apiToken},
		}),
	)

	if err != nil {
		return fmt.Errorf("failed to initialize Unleash client: %w", err)
	}

	log.Info("Unleash client initialized successfully")

	// Mark as initialized
	initMu.Lock()
	initialized = true
	initMu.Unlock()

	return nil
}

// Close gracefully shuts down the Unleash client
// Should be called during application shutdown (typically with defer)
func Close() error {
	err := unleash.Close()

	// Mark as not initialized
	initMu.Lock()
	initialized = false
	initMu.Unlock()

	return err
}

// IsInitialized returns whether Unleash successfully initialized
func IsInitialized() bool {
	initMu.RLock()
	defer initMu.RUnlock()
	return initialized
}

// IsEnabled checks if a feature flag is enabled
// Returns false if Unleash is not initialized
func IsEnabled(featureName string) bool {
	return unleash.IsEnabled(featureName)
}

// IsEnabledWithContext checks if a feature flag is enabled with Unleash context
// Context allows for per-organization gradual rollout and targeting
func IsEnabledWithContext(featureName string, uctx ucontext.Context) bool {
	return unleash.IsEnabled(featureName, unleash.WithContext(uctx))
}

// GetVariant gets a variant for a feature flag without context
// Returns a variant with Name="disabled" and Enabled=false if:
//   - Unleash is not initialized
//   - Feature flag doesn't exist
//   - Feature flag is disabled
func GetVariant(featureName string) *api.Variant {
	return unleash.GetVariant(featureName)
}

// GetVariantWithContext gets a variant for a feature flag with Unleash context
// Context allows for per-organization gradual rollout and targeting
func GetVariantWithContext(featureName string, uctx ucontext.Context) *api.Variant {
	return unleash.GetVariant(featureName, unleash.WithVariantContext(uctx))
}
