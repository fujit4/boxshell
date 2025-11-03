package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	ModeLinux   = "linux"
	ModeWindows = "windows"
)

// Config はアプリケーションの設定を保持します。
type Config struct {
	PathMode string `json:"path_mode"`
}

// DefaultConfig はデフォルトの設定を返します。
func DefaultConfig() *Config {
	return &Config{
		PathMode: ModeLinux,
	}
}

func getConfigPath() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataHome = filepath.Join(homeDir, ".local", "share")
	}
	return filepath.Join(dataHome, "boxshell", "config.json"), nil
}

// Load は設定ファイルを読み込みます。ファイルが存在しない場合はデフォルト設定を返します。
func Load() (*Config, error) {
	path, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}
	defer file.Close()

	cfg := &Config{}
	if err := json.NewDecoder(file).Decode(cfg); err != nil {
		return nil, err
	}

	// 不正な値が設定されていた場合はデフォルトに戻す
	if cfg.PathMode != ModeLinux && cfg.PathMode != ModeWindows {
		cfg.PathMode = ModeLinux
	}

	return cfg, nil
}

// Save は設定をファイルに保存します。
func (c *Config) Save() error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewEncoder(file).Encode(c)
}
