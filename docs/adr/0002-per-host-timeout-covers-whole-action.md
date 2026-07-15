# Per-host timeout covers the whole action

gossh treats `-timeout` as the maximum duration for one target host action, including remote preflight checks, SSH command execution, and file transfer. This gives batch operators a predictable upper bound per target host instead of exposing lower-level connection or read timeout details.

**Consequences**

Large file transfers must set a larger `-timeout` explicitly, while timeout failures should be reported as timeout failures rather than being translated into file existence or directory validation errors.
