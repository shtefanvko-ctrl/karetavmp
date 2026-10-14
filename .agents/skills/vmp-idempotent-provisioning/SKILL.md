---
name: vmp-idempotent-provisioning
description: Implement or audit repeatable KARETA runtime installation/update on a managed node.
version: 1.0.0
---
Use explicit desired version/ref, staged download/install, backup/rollback point and atomic switch where possible. Re-running after interruption must converge without duplicating services or corrupting configuration.
