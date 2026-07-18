---
id: WI-033
title: Redesign the Gateway administration experience
status: in_progress
priority: P1
owner: "gateway-ui-redesign"
depends_on: [WI-032]
parallel_group: G
touches:
  - internal/gateway/admin
  - docs/gateway.md
  - spec/roadmap.md
  - spec/work-items/WI-017-amd64-linux-synology-distribution.md
---

# WI-033 - Redesign the Gateway administration experience

## Outcome

The local Gateway administration surface feels like a deliberate NAS control
application rather than a raw form: it uses a polished, Synology-inspired
desktop shell, clear navigation and status hierarchy, responsive layouts, and
useful empty states while preserving every WI-032 security and behavior
contract.

## Scope

- Replace the single scrolling form with a compact top bar, persistent desktop
  navigation, focused content views, and overview status cards.
- Give first-run setup and login a dedicated centered authentication layout
  that clearly separates Gateway identity from DSM/NAS identity.
- Organize NAS profiles, MCP access, high-risk approvals, audit, and local
  administrator settings into distinct navigable views without changing their
  APIs or authority.
- Add intentional empty, loading, success, and error presentation. Empty lists
  remain useful rather than looking broken.
- Preserve Traditional Chinese copy, add clear form labels and keyboard focus,
  and make the status/message region accessible to assistive technology.
- Provide responsive desktop and narrow-screen layouts using only embedded
  HTML/CSS/JavaScript so the scratch image and offline SPK remain self-contained.

## Non-goals

- Copying Synology trademarks, logos, icons, or proprietary DSM assets.
- Changing Gateway authentication, NAS enrollment, MCP-token, approval, audit,
  state, schema, runtime, or DSM operation behavior.
- Adding a frontend framework, external CDN, font download, telemetry, theme
  marketplace, multi-admin roles, or a dark theme in this item.

## Design constraints

- The UI may be visually inspired by modern NAS administration products but
  remains clearly branded as `dsmctl Gateway`.
- Existing endpoint paths, request headers, cookies, session security, and
  secret non-disclosure remain unchanged.
- Stable element IDs used by the existing JavaScript and tests are preserved or
  deliberately updated with matching tests.
- No external network request is required to render or operate the UI.
- Destructive actions retain explicit confirmation and a distinct danger style.
- The hosting NAS remains an ordinary explicit profile; UI hierarchy must not
  imply automatic host ownership or privilege.

## Acceptance criteria

- [ ] Setup and login render as polished, focused entry screens with the
      one-hour/reset guidance and no DSM identity implication.
- [ ] Authenticated administration uses a top bar, navigation, overview, and
      separate NAS, MCP-token, approval, audit, and administrator views.
- [ ] All existing administrator actions remain reachable and preserve current
      API behavior, confirmations, authentication, and secret handling.
- [ ] Empty Profiles, Tokens, and Approvals show purposeful empty states and do
      not produce JavaScript errors.
- [ ] Desktop and narrow-screen layouts are usable without clipped controls or
      horizontal page overflow.
- [ ] Interactive controls have visible labels/focus, status messages are
      announced, and destructive buttons remain visually distinct.
- [ ] The page contains no external asset dependency and keeps the existing CSP
      compatible with its inline implementation.
- [ ] Admin UI tests, `go test ./... -count=1`, `go vet ./...`, Docker build,
      and browser walkthrough of setup/login/dashboard pass.
- [ ] User documentation describes the reorganized administration views.

## Verification

- Unit tests verify the rendered UI structure, navigation targets, security
  copy, stable controls, and absence of external assets/legacy credentials.
- Run `go test ./... -count=1`, `go vet ./...`, and `git diff --check`.
- Build the current `linux/amd64` image and exercise setup, navigation, logout,
  and login in an isolated localhost container.
- Capture desktop and narrow-screen screenshots. No live NAS call or mutation
  is authorized or required.

## Coordination

WI-017 remains responsible for real DSM portal and hardware certification. It
must certify this final UI rather than the superseded WI-032 presentation. This
item does not change any Synology package lifecycle or DSM operation code.

## Handoff

Fill this only when pausing incomplete work.
