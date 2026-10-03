package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

//go:embed bootstrap.sh
var bootstrapScript []byte

type Orchestrator struct {
	cfg Config
}

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type VerifyReport struct {
	NodeID string  `json:"node_id"`
	OK     bool    `json:"ok"`
	Checks []Check `json:"checks"`
}

func NewOrchestrator(cfg Config) *Orchestrator {
	return &Orchestrator{cfg: cfg}
}

func (o *Orchestrator) Auto(ctx context.Context) (VerifyReport, error) {
	if err := o.Install(ctx); err != nil {
		return VerifyReport{}, err
	}
	if o.cfg.Update.Auto {
		if err := o.Update(ctx); err != nil {
			return VerifyReport{}, err
		}
	}
	if err := o.Start(ctx); err != nil {
		return VerifyReport{}, err
	}
	return o.Verify(ctx)
}

func (o *Orchestrator) Install(ctx context.Context) error {
	if runtime.GOOS != "windows" {
		return errors.New("KARETA Node bootstrap must be run from Windows")
	}

	if _, err := exec.LookPath("wsl.exe"); err != nil {
		fmt.Println("[KARETA] WSL is absent; enabling WSL platform...")
		if _, runErr := runCommand(ctx, "wsl.exe", "--install", "--no-distribution"); runErr != nil {
			return fmt.Errorf("install WSL: %w", runErr)
		}
		return errRebootRequired
	}

	if !distroExists(ctx, o.cfg.WSL.Distro) {
		fmt.Println("[KARETA] Installing WSL distro:", o.cfg.WSL.Distro)
		if _, err := runCommand(ctx, "wsl.exe", "--install", "-d", o.cfg.WSL.Distro); err != nil {
			return fmt.Errorf("install WSL distro: %w", err)
		}
	}

	if err := o.ensureSystemd(ctx); err != nil {
		return err
	}

	fmt.Println("[KARETA] Bootstrapping Linux runtime...")
	if err := o.runBootstrap(ctx); err != nil {
		return err
	}

	if token, source, err := o.cfg.token(); err == nil {
		fmt.Println("[KARETA] Provisioning Cloudflare credential from", source)
		if err := o.writeToken(ctx, token); err != nil {
			return err
		}
	} else {
		fmt.Println("[KARETA] WARN: Cloudflare token not provisioned; tunnel will remain offline")
	}
	return nil
}

func (o *Orchestrator) Update(ctx context.Context) error {
	if !distroExists(ctx, o.cfg.WSL.Distro) {
		return errors.New("configured WSL distro is not installed")
	}
	fmt.Println("[KARETA] Applying idempotent runtime/project update...")
	return o.runBootstrap(ctx)
}

func (o *Orchestrator) Start(ctx context.Context) error {
	if _, err := o.runWSL(ctx, "root", "systemctl start kareta.target"); err != nil {
		return fmt.Errorf("start kareta.target: %w", err)
	}
	return nil
}

func (o *Orchestrator) Repair(ctx context.Context) error {
	if err := o.Install(ctx); err != nil {
		return err
	}
	return o.Start(ctx)
}

func (o *Orchestrator) Verify(ctx context.Context) (VerifyReport, error) {
	info := detectHost(ctx, o.cfg)
	report := VerifyReport{NodeID: info.NodeID, OK: true}

	add := func(name string, ok bool, detail string) {
		status := "PASS"
		if !ok {
			status = "FAIL"
			report.OK = false
		}
		report.Checks = append(report.Checks, Check{Name: name, Status: status, Detail: strings.TrimSpace(detail)})
	}

	add("windows", runtime.GOOS == "windows", runtime.GOOS)
	add("administrator", info.Admin, "")
	add("wsl", info.WSLPresent, info.WSLVersion)
	add("distro", info.DistroReady, o.cfg.WSL.Distro)

	if !info.DistroReady {
		return report, errors.New("WSL distro is not ready")
	}

	lock, lockErr := loadCloudflaredLock()
	var pinnedAsset CloudflaredAsset
	if lockErr == nil {
		if arch, archErr := o.runWSL(ctx, "root", "uname -m"); archErr == nil {
			pinnedAsset, lockErr = selectCloudflaredAsset(lock, arch.Output)
		} else {
			lockErr = archErr
		}
	}
	add("cloudflared_pin", lockErr == nil, func() string {
		if lockErr != nil {
			return lockErr.Error()
		}
		return lock.Source.Release + " " + pinnedAsset.Name
	}())

	for _, item := range []struct {
		name string
		cmd  string
	}{
		{"systemd", "systemctl --version >/dev/null"},
		{"php", "php -v | head -n1"},
		{"pdo_mysql", "php -r 'exit(in_array(\"mysql\", PDO::getAvailableDrivers(), true) ? 0 : 1);'"},
		{"nginx", "systemctl is-active nginx"},
		{"database", "(systemctl is-active mysql || systemctl is-active mariadb)"},
		{"kareta_ready_unit", "systemctl is-active kareta-ready.service"},
		{"messaging_worker", "systemctl is-active kareta-messaging-worker.service"},
		{"cloudflared_binary", "command -v cloudflared"},
		{"cloudflared_version", "cloudflared --version"},
		{"cloudflared", "systemctl is-active kareta-cloudflared.service"},
	} {
		r, err := o.runWSL(ctx, "root", item.cmd)
		add(item.name, err == nil, r.Output)
	}
	if lockErr == nil {
		r, err := o.runWSL(ctx, "root", "printf '%s  %s\\n' "+shellSingleQuote(pinnedAsset.SHA256)+" /usr/local/bin/cloudflared | sha256sum -c -")
		add("cloudflared_sha256", err == nil, r.Output)
	}

	localHealth := shellSingleQuote(o.cfg.Runtime.LocalURL + o.cfg.Runtime.RuntimeHealthPath)
	r, err := o.runWSL(ctx, "root", "curl -fsS --max-time 10 "+localHealth)
	add("runtime_health", err == nil, truncate(r.Output, 500))

	localReady := shellSingleQuote(o.cfg.Runtime.LocalURL + o.cfg.Runtime.ReadinessPath)
	r, err = o.runWSL(ctx, "root", "curl -fsS --max-time 15 "+localReady)
	readyOK := err == nil && jsonOK(r.Output)
	add("readiness", readyOK, truncate(r.Output, 800))

	if strings.TrimSpace(o.cfg.Cloudflare.Hostname) != "" {
		external := shellSingleQuote("https://" + o.cfg.Cloudflare.Hostname + o.cfg.Runtime.RuntimeHealthPath)
		r, err = o.runWSL(ctx, "root", "curl -fsS --max-time 15 "+external)
		add("external_tunnel", err == nil, truncate(r.Output, 500))
	}

	if !report.OK {
		return report, errors.New("verification failed")
	}
	return report, nil
}

func (o *Orchestrator) ensureSystemd(ctx context.Context) error {
	if _, err := o.runWSL(ctx, "root", "systemctl --version >/dev/null 2>&1 && test \"$(ps -p 1 -o comm= | tr -d ' ')\" = systemd"); err == nil {
		return nil
	}

	fmt.Println("[KARETA] Enabling systemd in WSL...")
	content := "[boot]\nsystemd=true\n"
	cmd := "umask 022; printf %s " + shellSingleQuote(content) + " > /etc/wsl.conf"
	if _, err := o.runWSL(ctx, "root", cmd); err != nil {
		return fmt.Errorf("write /etc/wsl.conf: %w", err)
	}
	if _, err := runCommand(ctx, "wsl.exe", "--terminate", o.cfg.WSL.Distro); err != nil {
		return fmt.Errorf("restart WSL distro: %w", err)
	}
	time.Sleep(2 * time.Second)
	if _, err := o.runWSL(ctx, "root", "systemctl --version >/dev/null 2>&1"); err != nil {
		return fmt.Errorf("systemd did not become available: %w", err)
	}
	return nil
}

func (o *Orchestrator) runBootstrap(ctx context.Context) error {
	lock, err := loadCloudflaredLock()
	if err != nil {
		return err
	}
	archResult, err := o.runWSL(ctx, "root", "uname -m")
	if err != nil {
		return fmt.Errorf("detect WSL architecture: %w", err)
	}
	asset, err := selectCloudflaredAsset(lock, archResult.Output)
	if err != nil {
		return err
	}
	fmt.Printf("[KARETA] cloudflared pin: %s %s sha256=%s\n", lock.Source.Release, asset.Name, asset.SHA256)

	args := []string{
		"-d", o.cfg.WSL.Distro,
		"--user", "root",
		"--",
		"bash", "-s", "--",
		o.cfg.Kareta.Repository,
		o.cfg.Kareta.Ref,
		o.cfg.Kareta.InstallPath,
		o.cfg.Runtime.LocalURL,
		o.cfg.Runtime.RuntimeHealthPath,
		o.cfg.Runtime.ReadinessPath,
		o.cfg.Cloudflare.Hostname,
		o.cfg.Cloudflare.TokenFileLinux,
		lock.Source.Release,
		asset.URL,
		asset.SHA256,
		fmt.Sprintf("%d", asset.Size),
	}
	r, err := runCommandInput(ctx, bytes.NewReader(bootstrapScript), "wsl.exe", args...)
	if r.Output != "" {
		fmt.Println(r.Output)
	}
	if err != nil {
		return fmt.Errorf("bootstrap WSL runtime: %w", err)
	}
	return nil
}

func (o *Orchestrator) writeToken(ctx context.Context, token []byte) error {
	cmd := "umask 077; mkdir -p " + shellSingleQuote(parentDir(o.cfg.Cloudflare.TokenFileLinux)) +
		"; cat > " + shellSingleQuote(o.cfg.Cloudflare.TokenFileLinux) +
		"; chmod 600 " + shellSingleQuote(o.cfg.Cloudflare.TokenFileLinux)
	args := []string{"-d", o.cfg.WSL.Distro, "--user", "root", "--", "bash", "-lc", cmd}
	if _, err := runCommandInput(ctx, bytes.NewReader(token), "wsl.exe", args...); err != nil {
		return fmt.Errorf("provision Cloudflare token: %w", err)
	}
	return nil
}

func (o *Orchestrator) runWSL(ctx context.Context, user, shellCommand string) (CommandResult, error) {
	return runCommand(ctx, "wsl.exe", "-d", o.cfg.WSL.Distro, "--user", user, "--", "bash", "-lc", shellCommand)
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func parentDir(p string) string {
	i := strings.LastIndex(p, "/")
	if i <= 0 {
		return "/"
	}
	return p[:i]
}

func jsonOK(raw string) bool {
	var v struct {
		OK bool `json:"ok"`
	}
	return json.Unmarshal([]byte(raw), &v) == nil && v.OK
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
