package config

import (
	"encoding/json"
	"strings"

	"github.com/ssdowd/couchbasebroker/utils"
)

type BoshConfig struct {
	DirectorURL      string `json:"director_url"`
	DirectorUser     string `json:"director_user"`
	DirectorPassword string `json:"director_password"`
	TemplateDir      string `json:"template_dir"`
	DataDir          string `json:"data_dir"`
}

type BoshOptions struct {
	instances int `json:"instances"`
}

var (
	currentBoshConfiguration BoshConfig
)

func LoadBoshConfig(path string) (*BoshConfig, error) {
	if !strings.HasPrefix(path, "/") {
		path = utils.GetPath([]string{path})
	}
	bytes, err := utils.ReadFile(path)
	if err != nil {
		currentBoshConfiguration = defaultBoshProperties()
		return &currentBoshConfiguration, err
	}

	err = json.Unmarshal(bytes, &currentBoshConfiguration)
	if err != nil {
		currentBoshConfiguration = defaultBoshProperties()
		return &currentBoshConfiguration, err
	}
	return &currentBoshConfiguration, nil
}

func GetBoshConfig() *BoshConfig {
	return &currentBoshConfiguration
}

func defaultBoshProperties() BoshConfig {
	return BoshConfig{
		DirectorURL:      "https://localhost:25555",
		DirectorUser:     "user",
		DirectorPassword: "password",
		TemplateDir:      "unknown",
	}
}
