# docs/ — v-rpc-tap

Standard vista-cloud-dev `docs/` layout. Do not invent per-repo folders
(`tracking/`, `plans/`, `prompts/`, `historical/`).

## Effort canon (lives in the central `docs` repo)

The scalable-tap **proposal** is **cross-cutting** — it drives `v-pkg` uninstall
changes and relates to v-stdlib's retired tap subsystem, and is inbound-linked
from several repos — so per org convention it stays in the central `docs` repo,
not here. This repo links to it rather than copying it:

- **Proposal (canonical, v3.6.0):**
  [`vista-cloud-dev/docs › proposals/v-rpc-tap-scalable.md`](https://github.com/vista-cloud-dev/docs/blob/main/proposals/v-rpc-tap-scalable.md)
- **Implementation tracker (live status, P0–P4 + L1–L11):**
  [`docs › proposals/v-rpc-tap-scalable-implementation-tracker.md`](https://github.com/vista-cloud-dev/docs/blob/main/proposals/v-rpc-tap-scalable-implementation-tracker.md)
- **Deep technical analysis (ground-truth-validated):**
  [`docs › proposals/considering/v-rpc-tap-scalable-deep-technical-analysis.md`](https://github.com/vista-cloud-dev/docs/blob/main/proposals/considering/v-rpc-tap-scalable-deep-technical-analysis.md)
- **Adversarial review:**
  [`docs › proposals/considering/v-rpc-tap-scalable-adversarial-review.md`](https://github.com/vista-cloud-dev/docs/blob/main/proposals/considering/v-rpc-tap-scalable-adversarial-review.md)
- **R12 dual-engine semantics probe:**
  [`docs › proposals/v-rpc-tap-r12-probe/`](https://github.com/vista-cloud-dev/docs/tree/main/proposals/v-rpc-tap-r12-probe)
- **Shared coordination memory:**
  [`docs › memory/v-rpc-tap-scalable.md`](https://github.com/vista-cloud-dev/docs/blob/main/memory/v-rpc-tap-scalable.md)

## This repo's docs/

```
docs/
  README.md   # this index — the one navigation entry point
  guides/     # how-to for users of `v rpc-tap` (added when verbs land)
  design/     # this repo's own design notes / ADRs (optional)
  memory/     # auto-memory — DURABLE facts only (created when first written)
  archive/    # retired docs from THIS repo — git mv'd, never deleted
```

Live-work trackers sit in `docs/` root as `<effort>-tracker.md` and move to
`docs/archive/` when the effort lands. (The scalable-tap tracker is currently in
the central `docs` repo with the cross-cutting proposal; if this effort narrows
to a single-repo concern it can graduate here.)
