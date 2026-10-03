package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

type CommandResult struct {
	Output   string
	ExitCode int
	Duration time.Duration
}

func runCommand(ctx context.Context, name string, args ...string) (CommandResult, error) {
	return runCommandInput(ctx, nil, name, args...)
}

func runCommandInput(ctx context.Context, stdin io.Reader, name string, args ...string) (CommandResult, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	result := CommandResult{
		Output:   strings.TrimSpace(strings.ReplaceAll(buf.String(), "\x00", "")),
		ExitCode: 0,
		Duration: time.Since(start),
	}
	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if ok := errorAs(err, &exitErr); ok {
		result.ExitCode = exitErr.ExitCode()
		return result, fmt.Errorf("%s exited with code %d: %s", name, result.ExitCode, result.Output)
	}
	result.ExitCode = -1
	return result, fmt.Errorf("run %s: %w", name, err)
}

func errorAs(err error, target interface{}) bool {
	switch t := target.(type) {
	case **exec.ExitError:
		e, ok := err.(*exec.ExitError)
		if ok {
			*t = e
		}
		return ok
	default:
		return false
	}
}
