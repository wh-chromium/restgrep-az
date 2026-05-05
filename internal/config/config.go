package config

import (
	"encoding/json"
	"os"
)

type BackendConfig struct {
	Type         string `json:"type"`
	Organization string `json:"organization,omitempty"`
	Project      string `json:"project,omitempty"`
	Repo         string `json:"repo,omitempty"`
}

type Config struct {
	Backends []BackendConfig `json:"backends"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
