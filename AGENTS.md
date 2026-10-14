# KARETA VMP Node Manager Agent Kernel

- Goal is a portable, repeatable KARETA node bootstrap/orchestrator for Windows hosts.
- Provisioning and updates must be idempotent and recoverable; never assume a clean machine.
- Secrets/tunnel credentials stay outside Git and outside packaged defaults.
- Separate host preparation, WSL/systemd runtime, KARETA deployment, readiness verification and tunnel startup.
- Never report READY from process start alone; verify local application/readiness evidence.
- Use `.agents/skills/`; run `vmp-devil-review` for provisioning, service/autostart, secret handling, destructive cleanup or packaging changes.
