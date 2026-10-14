---
name: vmp-node-orchestrator
description: Continue KARETA VMP Node Manager work as a staged Windows/WSL bootstrap and recovery workflow.
version: 1.0.0
---
Map DETECT HOST -> PREPARE WSL/systemd -> INSTALL/UPDATE RUNTIME -> CONFIGURE SERVICES -> VERIFY LOCAL READINESS -> START TUNNEL -> VERIFY EXTERNAL HANDOFF. Make stages resumable and persist only non-secret state.
