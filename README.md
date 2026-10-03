# KARETA VMP Node Manager

Portable Windows bootstrap/orchestrator for a KARETA.KZ node.

Goal: build a single `KARETA-Node.exe` that detects the host, prepares WSL/systemd, installs or updates the KARETA runtime, verifies local readiness, and starts the configured Cloudflare tunnel without committing secrets to Git.

Status: bootstrap repository initialized. Implementation work happens on feature branches and is merged only after CI verification.
