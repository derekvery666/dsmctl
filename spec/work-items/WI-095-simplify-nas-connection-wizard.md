---
id: WI-095
title: Simplify NAS connection wizard and make SPK upgrades converge
status: done
priority: P1
owner: codex
depends_on: [WI-094]
parallel_group: G
touches:
  - internal/gateway/admin/ui.go
  - internal/gateway/admin/handler_test.go
  - docs/gateway-admin-guide.md
  - internal/buildinfo/buildinfo.go
  - deploy/container/Dockerfile
  - deploy/synology/SUPPORTED.md
  - README.md
  - docs/compatibility.md
  - docs/synology-package.md
  - deploy/synology/spk/INFO.template
  - deploy/synology/spk/conf/resource
  - deploy/synology/spk/package/project/compose.yaml.template
  - deploy/synology/spk/scripts/start-stop-status
  - deploy/synology/spk/scripts/postupgrade
  - deploy/synology/validate-spk.sh
  - spec/roadmap.md
---

# WI-095 - Simplify NAS connection wizard and make SPK upgrades converge

## Outcome

The Add NAS wizard asks only for information needed by ordinary users: a
profile name and DSM address. Profiles created through this wizard are always
managed, and the implementation uses the standard 30-second connection timeout
without exposing an unexplained advanced control. The same release also fixes
the reproducible SPK upgrade lifecycle failure so upgrading a running package
leaves the package, container, authentication bridge, and portal healthy
without a Package Center Repair action.

## Scope

- Remove the role and timeout controls from the shared NAS connection step.
- Always create new wizard profiles with the managed role.
- Preserve an existing profile's role and timeout when its connection URL is
  edited through the wizard.
- Keep destination-only roles and custom timeout values available through the
  existing non-wizard backend/configuration surfaces.
- Tighten the connection-step layout after removing the advanced controls.
- Diagnose the DSM resource/startup failure that occurs after an otherwise
  successful upgrade and correct the package lifecycle declarations/scripts.
- Make the release procedure use one fixed build, validation, upload, upgrade,
  and health-verification path.

## Non-goals

- No migration or role change for existing profiles.
- No removal of role or timeout fields from persistent state or APIs.
- No change to discovery, TLS trust, DSM login, or request timeout semantics.
- No reliance on Package Center Repair as an installation or upgrade step.

## Design constraints

- A zero stored timeout continues to mean the runtime's 30-second default.
- Wizard edits must not silently reset advanced values created outside the UI.
- The gateway's managed/target authorization boundary remains unchanged.

## Acceptance criteria

- [x] The connection step contains no role selector or timeout input.
- [x] A profile created by the wizard sends the managed role and default
      timeout semantics.
- [x] Editing a profile URL preserves its existing role and timeout.
- [x] Manual entry and discovered-device selection both reach the simplified
      connection step.
- [x] Upgrading from a running prior build directly to this build invokes a
      successful package startup without an intervening Repair action.
- [x] After that upgrade, `synopkg status`, the core and bridge health checks,
      and the `/dsmctl/` portal all report healthy.
- [x] Focused/full tests, `go vet ./...`, embedded JavaScript syntax checking,
      browser walkthrough, SPK validation, and `git diff --check` pass.

## Verification

- `go test ./internal/gateway/admin -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- Parse the embedded script with Node.
- Live browser walkthrough on the DSM package without creating a Profile.
- Live upgrade from a running previous build; do not use Repair after the new
  build is installed.
- Verify package status plus ports 18765/18766 and the `/dsmctl/` portal.
- `git diff --check`

The user explicitly authorized rebuilding and repeatedly upgrading this
experimental package on the lab NAS. No storage, network, firewall, account,
share, or other unrelated live mutation is authorized.

## Coordination

This is a focused continuation of WI-094 and overlaps the same embedded Admin
UI currently carrying WI-091/WI-092 changes under the same `codex` owner.
The user expanded this item after the 7.3.2-10 live upgrade reproduced the
existing start-failed/Repair requirement. Preserve all unrelated
dirty-worktree changes.

## Handoff

Completed 2026-07-22 on the DS3018xs lab NAS. The reproducible upgrade failure
was Container Manager deleting its per-container profile during forced
recreation, then rejecting the immediately following startup with `container
[dsmctl-gateway-gateway-1]'s profile should exist!`. The SPK now avoids forced
recreation and overrides the inherited health check to the actual
`127.0.0.1:18765` listener.

`7.3.2-10` was running before the test. The validated `7.3.2-11` artifact was
uploaded and installed through `synopkg install`; DSM returned
`installed_and_started` and the log recorded `start version 7.3.2-11
successfully` followed by `upgrade ... successfully`. No Repair action was
used after installing this build. Package status was running, the container
became healthy on image `dsmctl-gateway:7.3.2-11`, ports 18765 and 18766 both
returned `{"status":"ok"}`, and `/dsmctl/` served the Admin UI. The live
browser walkthrough reached the simplified connection step with only Profile
name and DSM URL and closed without creating the `192.0.2.1` test profile.

Artifact SHA-256:
`ed1641f6632f02526c6a6b8a11050c5ae4bd36be41ea850b320389ced47638fb`.

Verification passed:

- `go test ./internal/gateway/admin -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- Node parsing of the embedded Admin JavaScript
- `deploy/synology/validate-spk.sh` and `sha256sum -c dist/SHA256SUMS`
- Direct live upgrade, package/container/core/bridge/portal health checks
- Live browser walkthrough without creating a Profile
- `git diff --check` (line-ending warnings only)
