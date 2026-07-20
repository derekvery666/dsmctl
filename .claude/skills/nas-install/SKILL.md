---
name: nas-install
description: >-
  Bring up a factory-fresh, reset, or crashed Synology NAS end to end with
  dsmctl: detect its install state, install DSM (online, or offline by
  auto-downloading the matching .pat from Synology when the device has no
  internet), then create the first administrator with the password stored in the
  OS credential store. Use when asked to "install DSM", "set up a new NAS",
  "reinstall a broken NAS", "裝好一台全新/reset 的 NAS", "線上安裝 DSM", or when a
  discovered device reports state not_install / sys_crash / sys_migrat.
---

# Bring up a fresh Synology NAS (install DSM + first admin)

Goal: take a NAS that has no usable DSM and end with DSM installed and a first
administrator whose password is in the OS credential store. Everything is a
`dsmctl` invocation; this skill is the order of operations and the decision
points. **Installing DSM erases the device's disks — it is destructive and
irreversible. Confirm the target with the user before `--install`.**

## 1. Find the device and its Web Assistant URL

The install endpoints live on the Web Assistant port (5000 http / 5001 https),
not the DSM API. Get the address from the user, or discover it:

```
dsmctl discover                 # lists LAN Synology devices with their state
```

A device whose state is `not_install`, `sys_crash`, or `sys_migrat` needs
installing. `dsmctl discover` cannot tell an installed-but-no-admin box from a
configured one, but the install detector below is authoritative.

## 2. Detect state (read-only — always do this first)

```
dsmctl install --url http://<ip>:5000
```

This prints model, serial, disk count, the state, and whether an install is
available **online** (the device can reach Synology) or must be done **offline**
(the host downloads the `.pat`). It changes nothing. States:

- `not_install` — no DSM; fresh/reset hardware → install.
- `sys_crash` — DSM installed but broken → reinstall.
- `sys_migrat` — disks moved from another NAS → migrate (not yet automated).

If it prints "not reachable online", the command will fall back to downloading
the matching image; the exact `.pat` URL it would use is shown.

## 3. Install DSM (destructive — needs the user's OK)

Same command with `--install`. It refuses unless you retype the device serial,
or pass `--yes` for automation. It auto-chooses:

- **Online** when the device reports internet access (fastest; the device
  downloads DSM itself).
- **Offline** when the device has no internet: the *host* downloads the DSM
  image that matches the device's own flash build from Synology, then uploads it.
  Use `--pat <file>` to supply a local image instead.

```
dsmctl install --url http://<ip>:5000 --install            # prompts for serial
dsmctl install --url http://<ip>:5000 --install --yes      # automation
dsmctl install --url http://<ip>:5000 --install --pat /path/DSM_<model>_<build>.pat
```

The command triggers the install, polls progress, and waits for the NAS to
reboot and DSM to come up (detected by `SYNO.API.Info` answering at the https
setup URL). This takes several minutes (plus download/upload time offline); run
it in the background and monitor if the harness has a timeout. When it finishes
it prints the setup URL, e.g. `https://<ip>:5001`.

## 4. Create the first administrator

Once DSM is up it is in first-run setup. Create the admin; the generated
password is stored in the OS credential store and never printed:

```
dsmctl provision <profile-name> --url https://<ip>:5001 --admin-user <user> --insecure-skip-tls-verify
```

`--insecure-skip-tls-verify` accepts the device's fresh self-signed certificate
(a lab convenience; interactively you would pin it instead). The password lands
in Windows Credential Manager / macOS Keychain / Linux Secret Service under the
profile name; retrieve it later, at a terminal, with
`dsmctl auth password reveal --nas <profile-name>`.

## Notes and limits

- **Match the image to the model/build.** Offline install auto-derives the
  Synology URL from the device's reported model + flash build
  (`.../release/<ver>/<build>/DSM_<model>_<build>.pat`); an explicit `--pat`
  must be the right platform or the device rejects it.
- **Offline still needs internet on the host**, just not on the device.
- **Not yet automated:** `sys_migrat` (migration), and surfacing install through
  the gateway/MCP (it is CLI-only today).
- Protocol reference: the `/webman/*.cgi` install API is documented in the
  `dsm-web-assistant-install-api` memory and `internal/provision/install.go`.
