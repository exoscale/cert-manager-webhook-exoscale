package main

import (
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// Environment variable names for configuration overrides.
const (
	envPrefix = "EXOSCALE_"

	envAPIKey    = envPrefix + "API_KEY"
	envAPISecret = envPrefix + "API_SECRET"
	envTrace     = envPrefix + "API_TRACE"
	envDebug     = envPrefix + "DEBUG"
)

const (
	defaultAPIEnvironment = "api"      // production environment
	defaultAPIZone        = "ch-gva-2" // Geneva cloud zone
)

// Config structure holds [Exoscale API client] configuration.
// Client credentials (APIKey, APISecret) must be in the form of
// reference to k8s Secret resource.
//
// [Exoscale API client]: https://github.com/exoscale/egoscale
type Config struct {
	APIKeyRef      *v1.SecretKeySelector `json:"apiKeyRef,omitempty"`
	APISecretRef   *v1.SecretKeySelector `json:"apiSecretRef,omitempty"`
	APIEnvironment string                `json:"apiEnvironment,omitempty"`
	APIZone        string                `json:"apiZone,omitempty"`
}

// LoadConfig is a helper function that decodes JSON configuration into
// the typed config struct.
// Empty values in JSON configuration are replaced with defaults.
// If JSON configuration is nil, returns empty Config struct (without any defaults).
func LoadConfig(cfgJSON *extapi.JSON) (Config, error) {
	if cfgJSON == nil {
		return Config{}, nil
	}

	cfg := Config{
		APIEnvironment: defaultAPIEnvironment,
		APIZone:        defaultAPIZone,
	}

	err := json.Unmarshal(cfgJSON.Raw, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %w", err)
	}

	return cfg, nil
}
