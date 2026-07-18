# Portable amd64 MCP gateway

`dsmctl-gateway` exposes the existing application layer over stateless MCP
Streamable HTTP. The image is a platform-neutral `linux/amd64` image: it runs
under Docker Engine, Podman, or Synology Container Manager without changing
the binary. The future Synology SPK is a deployment wrapper around this same
image, not a separate DSM-specific build.

The managed gateway stores up to 32 NAS profiles in a transactional embedded
database and exposes an authenticated administration page at `/admin/`.
Profiles and credentials take effect without restarting the process. Its MCP
surface remains read-only until WI-016 supplies scoped remote tokens,
out-of-band approvals, and auditing; all `plan_*` and `apply_*` tools are still
omitted in this intermediate build.

## Session model

MCP transport requests are stateless and return JSON. The gateway does not
issue or rely on a durable MCP session ID and does not open a standalone SSE
stream. DSM connectivity is intentionally different: the existing runtime
manager lazily keeps one client and authenticated DSM session per configured
NAS profile. Calls to different NAS profiles may run concurrently, while the
Synology client continues to serialize authentication and retry a request once
after an expired DSM session.

Stopping the process drains bounded in-flight HTTP requests and then closes
all cached DSM sessions.

## Managed Compose startup

The checked-in Compose project publishes the gateway only on
`127.0.0.1:18765`. Prepare its local files from the repository root:

```console
cd deploy/container
mkdir -p data secrets
openssl rand 32 > secrets/master.key
openssl rand -hex 32 > secrets/bootstrap
openssl rand -hex 32 > secrets/dev-token
chmod 700 data secrets
chmod 600 secrets/master.key secrets/bootstrap secrets/dev-token
sudo chown -R 10001:10001 data secrets
docker compose up --build
```

Open `http://127.0.0.1:18765/admin/`, paste the value from
`secrets/bootstrap`, and save the returned administrator token in a password
manager. Bootstrap is permanently invalid after that transaction. The page
keeps the administrator token only in the current tab's `sessionStorage`.
Remove the bootstrap file after first use; later starts no longer require it.
After changing ownership, read the bootstrap value with
`sudo cat secrets/bootstrap`.

Add each NAS from the page and choose one of two TLS policies: `system_ca`, or
`pinned_fingerprint` with an explicitly confirmed SHA-256 leaf-certificate
fingerprint. Production managed mode has no skip-verification option. Sign in
through the NAS's own DSM page (the gateway stores the resulting SID,
SynoToken, and Noise resume keys), or use the bounded password/OTP enrollment
for an automation account. Web sessions resume headlessly and survive gateway
restarts. The container never reads the host's desktop OS keyring.

The relay is tested against the DSM protocol locally. If a particular DSM
release refuses a non-loopback `opener` origin, use password/OTP enrollment for
that NAS until its browser-origin behavior is verified and supported.

For a custom host name or LAN address, add it to `DSMCTL_ALLOWED_HOSTS` and add
the exact browser origin to `DSMCTL_ALLOWED_ORIGINS` before starting Compose.
If a reverse proxy changes the public origin used by the browser, pass
`--admin-public-url=https://gateway.example` as well.

The MCP URL is `http://127.0.0.1:18765/mcp`. Send the contents of
`secrets/dev-token` as `Authorization: Bearer <token>`. This credential remains
an explicit read-only bridge until WI-016. `/healthz` is local process liveness
and never contacts DSM. `/readyz` verifies the state schema, established admin,
mounted master key, and MCP token; it does not poll the NAS fleet.

To put a trusted reverse proxy in front of the loopback listener, explicitly
set:

- `DSMCTL_ALLOWED_HOSTS` to the HTTP host names accepted by the backend;
- `DSMCTL_ALLOWED_ORIGINS` to exact browser origins, if browser MCP clients are
  used; requests without an `Origin` header remain valid for non-browser MCP
  clients;
- `DSMCTL_TRUSTED_PROXIES` to proxy CIDR ranges whose `X-Forwarded-For` value
  may be used for request logging.

TLS termination belongs at that proxy. Do not publish the development gateway
directly to the Internet.

## Direct binary startup

The same executable can run on an ordinary amd64 Linux host:

```console
dsmctl-gateway \
  --listen=127.0.0.1:18765 \
  --state=/srv/dsmctl/gateway.db \
  --master-key-file=/run/secrets/master.key \
  --bootstrap-file=/run/secrets/bootstrap \
  --dev-read-only-token-file=/run/secrets/dsmctl-token \
  --allowed-hosts=localhost,127.0.0.1
```

Omit `--state` only to retain the WI-014 static-config development mode.
Managed startup fails closed for a missing, malformed, or wrong master key and
for missing first-use bootstrap material. At most 32 NAS profiles are accepted,
per-profile timeouts are capped at 120 seconds, and at most 8 MCP requests run
concurrently by default.

## State, backup, and secret references

`gateway.db` uses bbolt transactions and a versioned schema. A pre-migration
backup is created beside the database before forward migration; migration or
key validation failure keeps readiness false. Passwords, trusted-device IDs,
web-login sessions, and apply secrets are encrypted independently with
AES-256-GCM and authenticated profile/type metadata. The master key is never
copied into the database or its backup.

The admin API can create opaque `vault:<id>` apply-time references. Only the
application's apply-time resolver can decrypt those values; MCP results and
plan hashing see only the reference. Removing a NAS deletes its credentials by
default. With explicit credential retention, the API lists orphan metadata so
the administrator can later delete it.

## Container security and portability

The image is built with `CGO_ENABLED=0` for `linux/amd64`, contains a single
static executable and CA roots, runs as numeric UID/GID `10001`, and requires
no shell. The Compose project uses a read-only root filesystem, a 16 MiB
`/tmp` tmpfs, drops every Linux capability, enables `no-new-privileges`, and
applies process, memory, CPU, and log bounds.

Only `/data` and `/run/secrets` are mounted. The image has no Docker socket and
does not use host networking. It contains no `/usr/syno` or `/var/packages`
integration, `SYNOPKG_*` handling, DSM `authenticate.cgi` calls, Synology
package lifecycle logic, or Container Manager control calls. Those concerns
belong to the WI-017 Synology wrapper.
