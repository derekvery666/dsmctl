# Portable amd64 MCP gateway

`dsmctl-gateway` exposes the existing application layer over stateless MCP
Streamable HTTP. The image is a platform-neutral `linux/amd64` image: it runs
under Docker Engine, Podman, or Synology Container Manager without changing
the binary. The future Synology SPK is a deployment wrapper around this same
image, not a separate DSM-specific build.

WI-014 is an explicit developer preview. Its HTTP surface is read-only: all
`plan_*` and `apply_*` tools are omitted. Remote planning, applying, durable
tokens, per-NAS authorization, approval records, and auditing remain disabled
until WI-016 supplies that policy boundary.

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

## Development Compose startup

The checked-in Compose project publishes the gateway only on
`127.0.0.1:18765`. Prepare its local files from the repository root:

```console
cd deploy/container
cp config.example.json data/config.json
openssl rand -hex 32 > secrets/dev-token
cp secrets/dsm-passwords.env.example secrets/dsm-passwords.env
chmod 600 data/config.json secrets/dev-token secrets/dsm-passwords.env
docker compose up --build
```

Replace the example NAS address, username, and password. Each password
environment variable is resolved only when that NAS is first contacted; it is
never included in profile listings or logs. Environment passwords are a
development bridge, not the final package vault.

This preview has no enrollment flow and deliberately does not connect to a
host desktop keyring: a web-login session created by `dsmctl auth login` lives
in that desktop user's OS keyring and is never readable by the gateway, so a
`DSMCTL_PASSWORD_<NAME>` variable is currently the only way a gateway profile
can authenticate. The preview also cannot complete an interactive DSM OTP
challenge or persist a trusted-device credential. Use only a narrowly
privileged read-only automation account that can authenticate
non-interactively for development. WI-015 replaces this bridge with the
encrypted gateway vault: web-login session enrollment through the admin flow,
renewed headlessly via session resume, plus password/OTP enrollment for
automation accounts.

The MCP URL is `http://127.0.0.1:18765/mcp`. Send the contents of
`secrets/dev-token` as `Authorization: Bearer <token>`. `/healthz` is local
process liveness and never contacts DSM. `/readyz` checks that the local config
and mounted token remain readable and valid; it does not poll the NAS fleet.

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
  --config=/srv/dsmctl/config.json \
  --dev-read-only-token-file=/run/secrets/dsmctl-token \
  --allowed-hosts=localhost,127.0.0.1
```

Startup fails closed if the config, token, Host allowlist, or trusted-proxy
configuration is invalid. At most 32 NAS profiles are accepted, per-profile
timeouts are capped at 120 seconds, and at most 8 MCP requests run concurrently
by default.

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
