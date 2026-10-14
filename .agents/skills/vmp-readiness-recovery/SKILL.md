---
name: vmp-readiness-recovery
description: Verify local KARETA node readiness and recover safely from failed bootstrap/update stages.
version: 1.0.0
---
Readiness requires concrete local service/API/app checks, not only process existence. Classify failure by host/WSL/runtime/database/web/tunnel layer. Retry only safe stages; preserve logs and rollback state. Do not hide degraded state behind READY.
