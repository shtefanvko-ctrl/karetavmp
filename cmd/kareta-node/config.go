package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed config.default.json
var defaultConfigJSON []byte

type Config struct {
	Schema int `json:"schema"`
	Node struct {
		ID   string `json:"id"`
		Role string `json:"role"`
	} `json:"node"`
	WSL struct {
		Distro string `json:"distro"`
	} `json:"wsl"`
	Kareta struct {
		Repository  string `json:"repository"`
		Ref         string `json:"ref"`
		InstallPath string `json:"install_path"`
	} `json:"kareta"`
	Runtime struct {
		LocalURL          string `json:"local_url"`
		RuntimeHealthPath string `json:"runtime_health_path"`
		ReadinessPath     string `json:"readiness_path"`
	} `json:"runtime"`
	Cloudflare struct {
		Hostname         string `json:"hostname"`
		TokenEnv         string `json:"token_env"`
		TokenFileWindows string `json:"token_file_windows"`
		TokenFileLinux   string `json:"token_file_linux"`
	} `json:"cloudflare"`
	Update struct {
		Auto               bool `json:"auto"`
		VerifyBeforeSwitch bool `json:"verify_before_switch"`
		RollbackOnFailure  bool `json:"rollback_on_failure"`
	} `json:"update"`
}

func loadConfig(explicitPath string) (Config, string, error) {
	var cfg Config
	if err := json.Unmarshal(defaultConfigJSON, &cfg); err != nil {
		return Config{}, "", fmt.Errorf("decode embedded config: %w", err)
	}

	path := strings.TrimSpace(explicitPath)
	if path == "" {
		exe, err := os.Executable()
		if err == nil {
			candidate := filepath.Join(filepath.Dir(exe), "kareta-node.json")
			if _, statErr := os.Stat(candidate); statErr == nil {
				path = candidate
			}
		}
	}

	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return Config{}, path, fmt.Errorf("read config %s: %w", path, err)
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return Config{}, path, fmt.Errorf("decode config %s: %w", path, err)
		}
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, path, err
	}
	return cfg, path, nil
}

func (c Config) Validate() error {
	if c.Schema != 1 {
		return fmt.Errorf("unsupported config schema %d", c.Schema)
	}
	if strings.TrimSpace(c.WSL.Distro) == "" {
		return errors.New("wsl.distro is required")
	}
	if !strings.HasPrefix(c.Kareta.Repository, "https://github.com/") {
		return errors.New("kareta.repository must use an https://github.com/ URL")
	}
	if strings.TrimSpace(c.Kareta.Ref) == "" {
		return errors.New("kareta.ref is required")
	}
	if !strings.HasPrefix(c.Kareta.InstallPath, "/") {
		return errors.New("kareta.install_path must be an absolute Linux path")
	}
	if !strings.HasPrefix(c.Runtime.LocalURL, "http://127.0.0.1") &&
		!strings.HasPrefix(c.Runtime.LocalURL, "http://localhost") &&
		!strings.HasPrefix(c.Runtime.LocalURL, "https://127.0.0.1") &&
		!strings.HasPrefix(c.Runtime.LocalURL, "https://localhost") {
		return errors.New("runtime.local_url must point to localhost")
	}
	if strings.TrimSpace(c.Cloudflare.TokenFileLinux) == "" {
		return errors.New("cloudflare.token_file_linux is required")
	}
	return nil
}

func (c Config) token() ([]byte, string, error) {
	if name := strings.TrimSpace(c.Cloudflare.TokenEnv); name != "" {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return []byte(value), "environment:" + name, nil
		}
	}
	if p := strings.TrimSpace(c.Cloudflare.TokenFileWindows); p != "" {
		if !filepath.IsAbs(p) {
			exe, err := os.Executable()
			if err != nil {
				return nil, "", err
			}
			p = filepath.Join(filepath.Dir(exe), p)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, "", fmt.Errorf("read Cloudflare token file: %w", err)
		}
		if value := strings.TrimSpace(string(raw)); value != "" {
			return []byte(value), "file:" + p, nil
		}
	}
	return nil, "", errors.New("Cloudflare token is not provisioned")
}
