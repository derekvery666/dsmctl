---
name: nas-provision
description: >-
  Use whenever the user wants to set up, provision, initialize, bootstrap, or bring up a
  Synology NAS from a blank / factory / freshly-reset state end to end with dsmctl — install
  DSM if needed, create the first administrator account, and finish the DSM setup wizard.
  Trigger on phrases like "set up my NAS", "provision a new Synology", "initial / first-time
  NAS setup", "configure a fresh DSM", "bootstrap the NAS", "create the admin account", "get
  this NAS ready", "把 NAS 裝起來", "從零設定 NAS", "初始化 NAS", "設定管理員帳號", or any
  request to take an unconfigured DSM to a working, logged-in-able system. This skill owns the
  security model: the admin USERNAME is operator-decided (never auto-generated); the admin
  PASSWORD is generated locally by dsmctl, stored in the OS credential store, and retrievable
  ONLY by a human at a terminal via `dsmctl auth reveal-password` — never by the model and
  never by any MCP tool. Do NOT hand-run the DSM install / QuickStart CGIs for a full setup
  without following this skill.
---

# nas-provision — set up a Synology NAS end to end

Takes a Synology NAS from unconfigured to a working DSM with a first administrator whose
username the operator chooses and whose password dsmctl generates and owns. Verified live on
a DS918+ running DSM 7.3.1.

## Security model — READ FIRST (non-negotiable)

1. **Username is operator-decided.** `--admin-user <name>` is required; never invent it.
2. **Password is dsmctl-generated, never seen by the model.** `dsmctl provision` generates a
   strong password with `crypto/rand`, uses it in-process to create the admin, and stores it
   in the OS credential store. It is never printed, logged, returned, or placed in a plan.
3. **Retrieval is human-only.** The password comes back only through
   `dsmctl auth reveal-password`, which is gated by **isatty AND a typed NAS-name
   confirmation read from stdin**. A pipe fails the terminal check; a non-interactive caller
   (CI, an agent, the Claude Code Bash tool's pty) reads end-of-input at the prompt and is
   refused. **The model cannot retrieve the password — do not try, and do not read the keyring
   directly.** No MCP tool ever returns a password (deferred by design).
4. When you (the assistant) reach a point that needs the plaintext, **tell the human to run
   `dsmctl auth reveal-password --nas <name>` in their own terminal.**

## Preconditions

- The NAS must be reachable over **https on the DSM port (5001)**. Provision refuses a
  cleartext http target so the password is never sent in the clear.
- The NAS must be **in its DSM first-run setup window** (freshly installed / reset DSM, no
  administrator yet). If DSM is not installed, install it first (see "Installing DSM" below).
- A fresh NAS presents a self-signed certificate. For an explicitly isolated lab NAS, pass
  `--insecure-skip-tls-verify`; otherwise pin the certificate the way `dsmctl auth login`
  does.

## The one command (happy path)

```console
dsmctl provision <profile-name> \
    --admin-user <username> \
    --url https://<nas-ip>:5001 \
    --insecure-skip-tls-verify \
    --device-name <hostname> \
    --auto-update security
```

What it does, in order (all live-verified):

1. **EstablishSetupSession** — logs in as the built-in `admin` with an **empty password**
   (enabled only during the setup window) to obtain the `_SSID` session cookie that authorizes
   account creation. Without it, DSM rejects the create with **error 119**.
2. **CreateFirstAdmin** — a sequential, stop-on-error `SYNO.Entry.Request` compound:
   `SYNO.Core.User create {name, password}` → `SYNO.Core.Group.Member add {group:"administrators", name:[user]}`. Over https the password is a plaintext parameter (DSM's own wizard does the same; transport encryption only engages on http).
3. **Login (verify) + SavePassword** — logs in as the new admin to confirm it works (the
   postcondition), then stores the generated password in the OS credential store **before**
   anything else can fail, so the only copy is never lost.
4. **CompleteSetup** — the remaining wizard steps: set the DSM update policy
   (`SYNO.Core.Upgrade.Setting` + `SYNO.Core.Package.Setting`), opt out of analytics
   (`SYNO.Core.DataCollect` / `SYNO.ActiveInsight.Setting`), then
   **`SYNO.Core.QuickStart.Info hide_welcome`** — the call that marks the wizard finished so
   DSM stops presenting it on login.
5. **Harden** (best-effort) — auto-block on repeated failed logins, set the server name, and
   scramble + disable the built-in `admin` account.
6. **Write the profile** to the dsmctl config.

Then tell the human:

```console
dsmctl auth reveal-password --nas <profile-name>   # type the NAS name to confirm
```
and to log in at `https://<nas-ip>:5001` as `<username>` with the revealed password.

## Options

- `--auto-update security` (default) installs security hotfixes automatically (no surprise
  major-version reboots), `all` installs every update, `notify` only notifies.
- `--analytics` opts in to Synology device analytics / Active Insight; default **off**.
- `--length N` sets the generated-password length (default 24).

## Finishing an already-created admin

If the administrator already exists (a provision interrupted after account creation, or one
done out of band) run only the post-account wizard steps — it logs in with the **stored**
password (resolved internally, never printed):

```console
dsmctl provision <profile-name> --finish-only --auto-update security
```

## Installing DSM first (if the NAS has no DSM)

A factory / mode-2-reset NAS boots the **Web Assistant** (recovery) at `http://<ip>:5000`,
which reinstalls DSM. Its recovery CGIs (unauthenticated, no cookie) were reverse-verified:

- `GET  /webman/get_state.cgi` — model, serial, MAC, status (`sys_crash` = needs reinstall).
- `GET  /webman/lock_check.cgi` — install lock.
- `POST /webman/install.cgi?upload=true&status=sys_crash&localinstallreq=false&utctime=…&_dc=…`
  with a multipart field **`filename`** = the `.pat` firmware → uploads and starts the install.
- `GET  /webman/get_install_progress.cgi` — poll `{"stage":"install"}` → `{"stage":"Success"}`.
- The NAS then reboots **automatically**; poll for it to come back, then DSM boots into the
  setup window this skill provisions. (This install path is not yet a `dsmctl` subcommand; it
  can be driven with `curl` as above, then hand off to `dsmctl provision`.)

## What this skill deliberately skips

Optional, externally-dependent, or package-installing steps are left for the operator to do
later from the DSM UI: **signing in to / creating a Synology Account, QuickConnect, and
installing recommended packages** (e.g. Surveillance Station). They require an external account
or downloading code and are not appropriate for an unattended provision default.

## Environment caveat (Claude Code sandbox on Windows)

The agent's shell tools run in a sandbox where the **project directory is shared** with the
user's machine but **`%AppData%` / user-profile files are virtualized** (writes don't reach the
real profile). The **Windows Credential Manager is OS-unified**, so a provision run inside the
agent stores the password in the user's *real* vault (`cmdkey /list` shows
`dsmctl:password/<profile>`) but writes the config profile only into the sandbox. Fix without
any NAS reset: the user recreates the profile in their real terminal, then reveals —

```console
dsmctl nas add <profile-name> --url https://<nas-ip>:5001 --username <username> --insecure-skip-tls-verify
dsmctl auth reveal-password --nas <profile-name>
```

Prefer to run `dsmctl provision` directly in the user's own terminal so both the profile and
the credential land in their real environment.

## Failure handling

- **Only a successful admin creation consumes the setup window** — a failed create is safe to
  retry.
- `has_fail` in a compound response means a sub-step failed; the command reports which step and
  the DSM error code (e.g. 119 = missing setup session, 402 = password policy). 103 on an
  `admin`/empty login means setup is already finished.
- The generated password is stored before hardening/wizard steps run, so even if a later step
  fails the credential is not lost — the admin is created and usable, and the password is
  revealable.

## Verification

`go test ./internal/config/... ./internal/credentials/... ./internal/provision/...` covers the
config identity model, password generation, keyring-only reveal, and the compound wire format
(via `httptest`). Live provisioning is opt-in against a genuinely fresh / setup-window NAS; the
webman install and QuickStart request shapes are pinned from a real DSM 7.3.1 pass. See
`spec/work-items/WI-083-nas-provision.md`.
