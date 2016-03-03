package config

import (
	"encoding/json"
	"github.com/ssdowd/couchbasebroker/utils"
)

// Config holds the configuration info for the broker (port, data file path,
// catalog path, instances file, bindings file, rest ID/password).
type Config struct {
	Port                     string `json:"port"`
	DataPath                 string `json:"data_path"`
	CatalogPath              string `json:"catalog_path"`
	ServiceInstancesFileName string `json:"service_instances_file_name"`
	ServiceBindingsFileName  string `json:"service_bindings_file_name"`
	RestUser                 string `json:"restuser"`
	RestPassword             string `json:"restpassword"`
}

var (
	currentConfiguration Config
)

// LoadConfig loads and returns the configuration at the indicated path.
func LoadConfig(path string) (*Config, error) {
	bytes, err := utils.ReadFile(path)
	if err != nil {
		return &currentConfiguration, err
	}

	err = json.Unmarshal(bytes, &currentConfiguration)
	if err != nil {
		return &currentConfiguration, err
	}
	return &currentConfiguration, nil
}

// GetConfig returns the current configuration.
func GetConfig() *Config {
	return &currentConfiguration
}
