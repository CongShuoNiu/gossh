# Non-zero exit code on target host failure

gossh returns a non-zero process exit code when any target host action fails or times out. Operators can still inspect per-host output for details, while scripts and automation platforms get a reliable process-level failure signal.

**Consequences**

Existing scripts that only parsed stdout and ignored the process exit code may observe a behavior change, but this aligns gossh with common command-line automation expectations.
