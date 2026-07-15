# Secure host key verification by default

gossh now verifies SSH target host identity through a known_hosts file by default, because batch operations can amplify the impact of connecting to the wrong host. This changes the older insecure-by-default behavior, so users who intentionally operate in trusted recovery networks must opt in with `-insecure-ignore-host-key`.

**Considered Options**

- Keep the old default and add strict verification as an option.
- Trust the first observed host key automatically.
- Verify host keys by default and require an explicit insecure opt-in.

**Consequences**

Users may need to create or pass a known_hosts file before first use, but accidental or malicious host impersonation is no longer accepted silently.
