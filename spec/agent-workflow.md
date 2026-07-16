# Multi-agent workflow

The repository may be edited by several agents, but each work item has one
active owner. Parallel work is encouraged only when ownership boundaries do not
overlap.

## Claiming an item

Before implementation, the agent must:

1. Read this file, `architecture-contracts.md`, and the entire work item.
2. Confirm all `depends_on` items are `done` or explicitly waived.
3. Change the work-item metadata to `status: in_progress` and set `owner`.
4. Update the matching roadmap row in the same commit.
5. Record any expected overlap under `coordination` and notify the other owner.

If metadata cannot be committed immediately, communicate the claim before
editing shared files. Do not infer ownership from an uncommitted worktree.

## File ownership

- A work item's `touches` list is a forecast, not exclusive permission.
- `internal/domain`, `internal/application`, `internal/mcpserver/server.go`, and
  compatibility reporting are high-contention areas. Coordinate before two
  active items edit the same file.
- New operation packages are the preferred parallel boundary.
- Avoid broad refactors inside a feature item. Create a prerequisite work item
  when a cross-cutting refactor is necessary.

## Implementation loop

1. Preserve unrelated user changes in a dirty worktree.
2. Add normalized models before adapters.
3. Add request-capture/decoder tests with the first operation variant.
4. Wire capability reporting and the facade.
5. Add application policy and only then CLI/MCP adapters.
6. Run focused tests during development and `go test ./...` plus `go vet ./...`
   before handoff.
7. Follow the work item's live-test policy exactly.

## Handoff and completion

An unfinished handoff must add a `handoff` section containing:

- last known good commit or working-tree state;
- completed acceptance criteria;
- failing command and exact error;
- temporary resources that still exist;
- assumptions that still need verification.

To mark an item `done`, the owner must:

1. Check every acceptance criterion or document an approved exception.
2. Update user docs and capability examples.
3. Set `status: done`, clear `owner`, and update the roadmap.
4. Add verification commands and, when applicable, the DSM versions tested.

## Commit guidance

Prefer one independently reviewable commit per work item. A prerequisite shared
refactor should be a separate commit so parallel agents can build on it without
cherry-picking an incomplete product surface.
