package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

//go:embed cloudflared.lock.json
var cloudflaredLockJSON []byte

type CloudflaredLock struct {
	Schema int `json:"schema"`
	Source struct {
		Repository   string `json:"repository"`
		Path         string `json:"path"`
		SourceCommit string `json:"source_commit"`
		Release      string `json:"release"`
	} `json:"source"`
	Assets map[string]CloudflaredAsset `json:"assets"`
}

type CloudflaredAsset struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	URL    string `json:"url"`
}

func loadCloudflaredLock() (CloudflaredLock, error) {
	var lock CloudflaredLock
	if err := json.Unmarshal(cloudflaredLockJSON, &lock); err != nil {
		return CloudflaredLock{}, fmt.Errorf("decode cloudflared lock: %w", err)
	}
	if lock.Schema != 1 {
		return CloudflaredLock{}, fmt.Errorf("unsupported cloudflared lock schema %d", lock.Schema)
	}
	if strings.TrimSpace(lock.Source.Release) == "" || len(lock.Source.SourceCommit) != 40 {
		return CloudflaredLock{}, errors.New("invalid cloudflared lock provenance")
	}
	for _, key := range []string{"linux_amd64", "linux_arm64"} {
		a, ok := lock.Assets[key]
		if !ok {
			return CloudflaredLock{}, fmt.Errorf("cloudflared lock missing %s", key)
		}
		if a.Size <= 0 || len(a.SHA256) != 64 || !strings.HasPrefix(a.URL, "https://github.com/cloudflare/cloudflared/releases/download/"+lock.Source.Release+"/") {
			return CloudflaredLock{}, fmt.Errorf("invalid cloudflared asset %s", key)
		}
	}
	return lock, nil
}

func selectCloudflaredAsset(lock CloudflaredLock, unameMachine string) (CloudflaredAsset, error) {
	switch strings.ToLower(strings.TrimSpace(unameMachine)) {
	case "x86_64", "amd64":
		return lock.Assets["linux_amd64"], nil
	case "aarch64", "arm64":
		return lock.Assets["linux_arm64"], nil
	default:
		return CloudflaredAsset{}, fmt.Errorf("unsupported WSL architecture %q for pinned cloudflared runtime", unameMachine)
	}
}
