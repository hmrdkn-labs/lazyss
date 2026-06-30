# ADR 0001: LazySS V1 Architecture

LazySS uses pragmatic DDD boundaries: pure domain types, app-layer use cases,
owned ports, and adapters for SSH config, AWS SSM, state storage, doctor checks,
and the Bubble Tea TUI.

The root repository ignores the local `lazyssh/` and `lazyssm/` reference
checkouts. Code is migrated by behavior and tests, not by wholesale merging.

V1 treats health as local, method-specific observations. It is not a daemon and
does not claim provider SLA uptime.
