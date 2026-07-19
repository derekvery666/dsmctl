---
id: WI-037
title: Unify Gateway color design tokens
status: done
priority: P1
owner: ""
depends_on: [WI-035]
parallel_group: G
touches:
  - internal/gateway/admin
  - docs/gateway.md
  - docs/gateway-admin-guide.md
  - spec/roadmap.md
  - spec/work-items/WI-017-amd64-linux-synology-distribution.md
---

# WI-037 - Unify Gateway color design tokens

## Outcome

The authentication experience and administration shell use one coherent brand
blue ramp and one neutral slate ramp. Bright blue, deep blue, and dark surfaces
feel related rather than independently selected.

## Scope

- Define explicit brand-blue and slate color scales in the embedded UI root.
- Express primary actions, focus, selected navigation, authentication artwork,
  logo, overview artwork, links, and dark navigation through semantic aliases
  backed by the same scales.
- Replace hard-coded blue and blue-gray values in component rules with tokens.
- Retain separate semantic success, warning, and danger colors.
- Produce the final per-page mockups and tutorial from the tokenized UI.

## Non-goals

- Changing layout, typography, content, localization, APIs, or behavior.
- Adding theming, a dark mode, external CSS, or proprietary Synology assets.

## Design constraints

- The dark navigation surface uses the deepest brand-blue tokens, not an
  unrelated gray hue.
- The authentication artwork uses the same deep brand-blue end as navigation;
  bright blue is reserved for actions, selection, and focus. Content surfaces
  remain light for dense form and table readability rather than becoming a
  full dark mode.
- Text and neutral borders use slate tokens; status colors retain their semantic
  meaning and must not be replaced with brand blue.
- Existing accessibility, responsive, offline, and CSP contracts remain
  unchanged.

## Acceptance criteria

- [x] Authentication artwork, logo, primary controls, navigation, and overview
      artwork are derived from one documented brand-blue scale.
- [x] Text, muted copy, borders, canvas, and neutral surfaces are derived from
      one slate scale and semantic surface aliases.
- [x] Component CSS no longer contains independent legacy blue/navy aliases or
      hard-coded blue-gray values except deliberate translucent overlays.
- [x] Existing localized UI behavior, security, actions, and responsive layout
      remain unchanged.
- [x] UI tests, full Go tests, vet, Docker build, and browser walkthrough pass.
- [x] Final mockups cover setup, login, overview, NAS, MCP access, approvals,
      Audit, and administrator pages; a concise Traditional Chinese tutorial is
      delivered with them.

## Verification

- Assert the rendered UI contains the shared design-token scales and semantic
  aliases.
- Run `go test ./... -count=1`, `go vet ./...`, and `git diff --check`.
- Build and exercise an isolated `linux/amd64` localhost container. No NAS
  connection or DSM mutation is authorized or required.

## Coordination

WI-017 owns distribution and hardware certification. It must certify the final
tokenized UI image; WI-037 changes presentation only.

## Completion notes

- Added documented `brand-50` through `brand-950` and `slate-25` through
  `slate-950` scales plus semantic action, focus, navigation, surface, text,
  border, and status aliases. Legacy `--blue` and `--navy` aliases and
  component-level blue-gray literals were removed.
- Authentication artwork and navigation now use the same deep brand-blue end;
  primary actions, selection, and focus use the brighter end. Dense content
  remains on light slate surfaces.
- Added rendered-UI assertions for the shared token contract and a concise
  Traditional Chinese administrator guide covering initialization, multi-NAS
  enrollment, MCP scopes, approvals, Audit, and administrator sessions.
- An isolated `linux/amd64` image (`dsmctl-gateway:ui-wi036`, local image ID
  `sha256:a8dad1d9c626b245f087832cc49775c3546b8b23bc75efd569f70b1edcebaa59`)
  passed setup, login, two-profile creation, Token creation, approval creation,
  every management view, logout, `/healthz`, and `/readyz`. No NAS connection
  or DSM mutation was performed.
- Final mockups for setup, login, overview, NAS, MCP access, approvals, Audit,
  and administrator views are versioned under `docs/assets/gateway-admin/`
  and embedded in `docs/gateway-admin-guide.md`. `go test ./... -count=1`,
  `go vet ./...`, and `git diff --check` pass.

## Handoff

Fill this only when pausing incomplete work.
