# Repository agent instructions

Before starting planned feature work:

1. Read `spec/README.md`, `spec/architecture-contracts.md`, and
   `spec/agent-workflow.md`.
2. Select a `ready` item from `spec/roadmap.md` and read its complete work-item
   file.
3. Claim it by setting `status: in_progress` and `owner` in the work item, then
   update the roadmap row in the same change.
4. Do not work around an unmet `depends_on` item without an explicit decision
   recorded in the work item.

Preserve the shared CLI/MCP application boundary and operation-scoped DSM
compatibility rules in `spec/architecture-contracts.md`.

Never run storage-pool, volume, SAN target/mapping, encrypted-share, WORM,
network, firewall, or other disruptive live mutations without explicit
authorization for that exact test. Disposable LUN create/delete is authorized
only for a unique `dsmctl-e2e-lun-*` LUN that is never mapped and is deleted
only after its stable DSM LUN ID is verified. Existing account/share live tests
may only use unique `dsmctl-e2e-*` resources and stable-ID-verified cleanup.

When handing work to another agent, update the work item's `Handoff` section
with the last known good state, verification results, blockers, and any
temporary resources that remain.
