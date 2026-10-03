# KARETA VMP Node Manager

Portable Windows bootstrap/orchestrator for a KARETA.KZ node.

The target artifact is a single `KARETA-Node.exe`. Double-click mode (`auto`) detects the PC, elevates through UAC when required, checks WSL, prepares the configured Ubuntu distro, bootstraps the KARETA runtime, provisions the Cloudflare tunnel credential from a protected source, starts `kareta.target`, and verifies local/external health.

## Current MVP

Implemented on branch `feat/kareta-node-mvp-20261003`:

- deterministic Node ID from Windows hostname + MachineGuid;
- embedded default configuration, with optional adjacent `kareta-node.json` override;
- commands: `auto`, `install`, `update`, `start`, `verify`, `repair`, `detect`;
- Windows UAC elevation;
- WSL distro detection and systemd enablement;
- idempotent Ubuntu bootstrap for nginx, PHP-FPM, MySQL/MariaDB and KARETA source;
- nginx parity rules for the current KARETA `.htaccess` contract;
- `kareta-ready.service`, messaging worker, Cloudflare service and `kareta.target`;
- Cloudflare token read from an environment variable or local token file and written inside WSL with mode 0600;
- verified cloudflared 2026.8.2 runtime selection for Linux amd64/arm64 from the KARETA_TUNNEL supply-chain contract;
- cloudflared is downloaded only from the pinned release URL, checked by exact byte size and SHA-256 before install, and checked again after install;
- Cloudflare runs with `--token-file`; secrets are never committed;
- verification of the cloudflared pin/hash plus PHP/PDO, DB, nginx, worker, readiness and external tunnel;
- CI builds `KARETA-Node.exe` as a GitHub Actions artifact. Binaries are not committed to source Git.

## Build

Windows:

```powershell
.\scripts\build.ps1
```

Or:

```powershell
go test ./...
$env:CGO_ENABLED="0"
$env:GOOS="windows"
$env:GOARCH="amd64"
go build -trimpath -o dist\KARETA-Node.exe .\cmd\kareta-node
```

## One-file mode

The EXE includes a safe staging default:

- repository: `shtefanvko-ctrl/kareta`;
- WSL distro: `Ubuntu`;
- origin: `http://127.0.0.1`;
- external hostname: `s.kareta.kz`.

A machine-specific config can override those values, but raw Cloudflare tokens are intentionally not part of the JSON schema. The token is accepted from `KARETA_CF_TUNNEL_TOKEN` or from a local file named by `cloudflare.token_file_windows`, then copied to the root-only WSL token file.

## Safety boundary

The bootstrap does not run database migrations or reset an existing dirty KARETA working tree. It fails closed instead. It never downloads a cloudflared `latest` artifact: the embedded lock mirrors the exact KARETA_TUNNEL runtime contract and unsupported WSL architectures fail closed.

Database/schema provisioning, Windows startup registration/checkpoint-resume, and live execution on the target KARETA PC remain separate verification increments.

