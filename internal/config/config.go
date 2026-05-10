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
	Limit        int    `json:"limit,omitempty"`
	BackendMode  string `json:"backend_mode,omitempty"` // "naive", "local", "git-diff"
}

type Config struct {
	Backends              []BackendConfig `json:"backends"`
	ExecutionMode         string          `json:"execution_mode,omitempty"`           // "parallel" or "sequential"
	InexactSHA1Adjustment bool            `json:"inexact_sha1_adjustment,omitempty"` // Global default
	BackendMode           string          `json:"backend_mode,omitempty"`             // Global default: "naive", "local", "git-diff"
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
