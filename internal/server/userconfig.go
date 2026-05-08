package server

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type UserConfig struct {
	APIKey   string `json:"api_key"`
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
}

func LoadUserConfig() UserConfig {
	home, err := os.UserHomeDir()
	if err != nil {
		return UserConfig{}
	}
	data, err := os.ReadFile(filepath.Join(home, ".coolcassette.json"))
	if err != nil {
		return UserConfig{}
	}
	var cfg UserConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return UserConfig{}
	}
	return cfg
}

func SaveUserConfig(cfg UserConfig) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(home, ".coolcassette.json"), data, 0600)
}
