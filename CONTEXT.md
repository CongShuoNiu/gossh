# gossh

gossh is a command-line SSH operations tool for single-host and batch host command execution and file transfer.

## Language

**Target Host**:
A remote machine that gossh connects to for command execution, file push, or file pull.
_Avoid_: Server, machine, node

**Batch Operation**:
One gossh invocation that applies the same command or file transfer action to multiple target hosts from an IP file.
_Avoid_: Parallel task, mass execution

**Batch Summary**:
The final aggregate result for a batch operation, including total target hosts, successful target hosts, failed target hosts, skipped target hosts, and failed target host identifiers.
_Avoid_: Report, statistics

**Host Key Verification**:
The SSH identity check that verifies a target host against a known_hosts file before authentication or command execution.
_Avoid_: Host check, SSH verification

**Known Hosts File**:
The file used as the trusted source for SSH host key verification.
_Avoid_: Key file, host list

**Per-Host Timeout**:
The maximum time allowed for one target host action within a gossh invocation.
_Avoid_: Global timeout, command timeout

**Transfer Metrics**:
The per-target-host SCP observability fields for transferred bytes, transfer duration, and average throughput.
_Avoid_: Progress, speed log
