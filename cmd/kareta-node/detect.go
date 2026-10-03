package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type HostInfo struct {
	Hostname    string `json:"hostname"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Admin       bool   `json:"admin"`
	MachineID   string `json:"machine_id"`
	NodeID      string `json:"node_id"`
	WSLPresent  bool   `json:"wsl_present"`
	WSLVersion  string `json:"wsl_version,omitempty"`
	DistroReady bool   `json:"distro_ready"`
}

func detectHost(ctx context.Context, cfg Config) HostInfo {
	hostname, _ := os.Hostname()
	info := HostInfo{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Admin:    isAdministrator(ctx),
	}

	if runtime.GOOS == "windows" {
		info.MachineID = windowsMachineGUID(ctx)
		_, err := exec.LookPath("wsl.exe")
		info.WSLPresent = err == nil
		if info.WSLPresent {
			if r, err := runCommand(ctx, "wsl.exe", "--version"); err == nil {
				info.WSLVersion = firstNonEmptyLine(r.Output)
			}
			info.DistroReady = distroExists(ctx, cfg.WSL.Distro)
		}
	}

	sum := sha256.Sum256([]byte(strings.ToLower(info.Hostname) + "|" + info.MachineID))
	short := strings.ToUpper(hex.EncodeToString(sum[:6]))
	if strings.TrimSpace(cfg.Node.ID) != "" && cfg.Node.ID != "auto" {
		info.NodeID = cfg.Node.ID
	} else {
		info.NodeID = "KARETA-" + short
	}
	return info
}

func windowsMachineGUID(ctx context.Context) string {
	if runtime.GOOS != "windows" {
		return ""
	}
	r, err := runCommand(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		"(Get-ItemPropertyValue -Path 'HKLM:\\SOFTWARE\\Microsoft\\Cryptography' -Name MachineGuid)")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(r.Output)
}

func isAdministrator(ctx context.Context) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	r, err := runCommand(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		"([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)")
	return err == nil && strings.EqualFold(strings.TrimSpace(r.Output), "true")
}

func distroExists(ctx context.Context, distro string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	r, err := runCommand(ctx, "wsl.exe", "--list", "--quiet")
	if err != nil {
		return false
	}
	want := strings.TrimSpace(distro)
	for _, line := range strings.Split(r.Output, "\n") {
		if strings.EqualFold(strings.TrimSpace(line), want) {
			return true
		}
	}
	return false
}

func elevateSelf() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("administrator elevation is only supported on Windows")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	quoted := make([]string, 0, len(os.Args)-1)
	for _, arg := range os.Args[1:] {
		quoted = append(quoted, "'"+strings.ReplaceAll(arg, "'", "''")+"'")
	}
	script := fmt.Sprintf(
		"Start-Process -FilePath '%s' -ArgumentList @(%s) -Verb RunAs",
		strings.ReplaceAll(exe, "'", "''"),
		strings.Join(quoted, ","),
	)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	return cmd.Start()
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if v := strings.TrimSpace(line); v != "" {
			return v
		}
	}
	return ""
}

func shortContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}
