package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

const version = "0.2.0"

var errRebootRequired = errors.New("Windows restart required")

func main() {
	command, configPath, jsonOutput, err := parseArgs(os.Args[1:])
	if err != nil {
		fatal(err)
	}

	cfg, loadedFrom, err := loadConfig(configPath)
	if err != nil {
		fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	info := detectHost(ctx, cfg)
	if requiresAdmin(command) && runtime.GOOS == "windows" && !info.Admin {
		fmt.Println("[KARETA] Administrator rights are required. Opening UAC...")
		if err := elevateSelf(); err != nil {
			fatal(fmt.Errorf("elevate: %w", err))
		}
		return
	}

	if loadedFrom == "" {
		fmt.Println("[KARETA] config: embedded")
	} else {
		fmt.Println("[KARETA] config:", loadedFrom)
	}
	fmt.Printf("[KARETA] version=%s node=%s role=%s host=%s\n", version, info.NodeID, cfg.Node.Role, info.Hostname)

	app := NewOrchestrator(cfg)
	var result any

	switch command {
	case "auto":
		result, err = app.Auto(ctx)
	case "install":
		err = app.Install(ctx)
	case "update":
		err = app.Update(ctx)
	case "start":
		err = app.Start(ctx)
	case "verify":
		result, err = app.Verify(ctx)
	case "repair":
		err = app.Repair(ctx)
	case "detect":
		result = info
	case "version":
		fmt.Println(version)
		return
	default:
		fatal(fmt.Errorf("unknown command %q", command))
	}

	if errors.Is(err, errRebootRequired) {
		fmt.Println("[KARETA] Windows restart is required. Run KARETA-Node.exe again after restart.")
		os.Exit(10)
	}
	if err != nil {
		fatal(err)
	}

	if result != nil {
		if jsonOutput {
			raw, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(raw))
		} else {
			fmt.Println(formatResult(result))
		}
	}
}

func parseArgs(args []string) (command, configPath string, jsonOutput bool, err error) {
	command = "auto"
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			jsonOutput = true
		case arg == "--config":
			if i+1 >= len(args) {
				return "", "", false, errors.New("--config requires a path")
			}
			i++
			configPath = args[i]
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		case strings.HasPrefix(arg, "-"):
			return "", "", false, fmt.Errorf("unknown option %s", arg)
		default:
			if command != "auto" {
				return "", "", false, fmt.Errorf("unexpected argument %s", arg)
			}
			command = strings.ToLower(arg)
		}
	}
	return
}

func requiresAdmin(command string) bool {
	switch command {
	case "auto", "install", "update", "start", "repair":
		return true
	default:
		return false
	}
}

func formatResult(v any) string {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(raw)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "[KARETA] FAIL:", err)
	os.Exit(1)
}
