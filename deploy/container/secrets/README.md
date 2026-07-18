# Gateway secrets

Create these owner-readable files before first start:

- `master.key`: exactly 32 random binary bytes; never place it in `/data` or a
  normal database backup;
- `bootstrap`: at least 32 random non-whitespace characters, used exactly once
  to establish the first local administrator;
- `dev-token`: at least 32 random non-whitespace characters for the temporary
  read-only MCP boundary (WI-016 replaces this with scoped tokens).

An optional `dsm-passwords.env` may retain the environment-password fallback
for narrowly scoped automation accounts, but the admin web-login/password+OTP
enrollment and encrypted vault are the normal managed path. Files in this
directory are mounted read-only and ignored by Git.
