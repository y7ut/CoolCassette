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

func loadConfigFile(path string) (UserConfig, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return UserConfig{}, false
	}
	var cfg UserConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return UserConfig{}, false
	}
	return cfg, true
}

func exeDirConfigPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "coolcassette.json")
}

func LoadUserConfig() UserConfig {
	home, err := os.UserHomeDir()
	if err == nil {
		if cfg, ok := loadConfigFile(filepath.Join(home, ".coolcassette.json")); ok {
			return cfg
		}
	}
	if p := exeDirConfigPath(); p != "" {
		if cfg, ok := loadConfigFile(p); ok {
			return cfg
		}
	}
	return UserConfig{}
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
