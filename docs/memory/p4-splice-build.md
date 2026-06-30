---
name: p4-splice-build
description: P4 step 1 — the XWBPRS foreign-overwrite splice build (internal/splice + rpc-tap splice verb + cmd/rpctap), the per-site regeneration flow, and the full v pkg install/restore lifecycle proven byte-clean on vehu (which also surfaced + got two v-pkg bugs fixed).
metadata:
  type: project
---

**P4 step 1 DONE (2026-06-30):** the tap now ships as a real foreign-overwrite KIDS
build — splices the national `XWBPRS` + ships greenfield `VSLRT*` + a `#19` OPTION —
installed/backed-out **exclusively via `v pkg`** (org hard rule [[never-use-bespoke-installer]]).

## Splice contract (pinned base **67, byte-identical vehu/foia)
`internal/splice.Splice([]string)` inserts the two bare tap calls (D7 — no logic in
the broker) into `CALLP^XWBPRS`, deriving each call's indent from its anchor:
- after the **XWBSEC denial line** (`S:$L($G(XWBSEC)) ERR="-1^"_XWBSEC`, CALLP+9) →
  top-level ` D req^VSLRTAP`
- after the **dotted `D CAPI` line** (CALLP+11) → dotted ` . D rsp^VSLRTAP` (stays
  inside the line-16 DO block)

It **REFUSES** (never splices blind) on: no CALLP label, a missing/ambiguous anchor,
or an already-tapped routine. Anchors match distinctive substrings (`ERR="-1^"_XWBSEC`;
a dotted line containing `D CAPI`) so trivial drift fails generation rather than
mis-splicing. Line 2 (patch string) is left untouched — versioning is the build's job.

## Regenerate-per-site, never a frozen copy (proposal §12)
The spliced XWBPRS is **build input**, not committed: `cmd/rpctap` (the standalone
`rpc-tap` binary) exposes `rpc-tap splice --in <site XWBPRS.m> --out <staged>`, and
`make kids SPLICE_SRC=<site XWBPRS.m>` stages it with the VSLRT* and runs `v pkg build`.
Pull the site's current XWBPRS read-only over the driver — **`m-ydb sync diff XWBPRS
--from <dir>`** (a fast single-routine compare; full `sync pull` times out on a whole
VistA). Confirmed vehu's live XWBPRS is byte-identical to the static `vista-m-host`
source (empty diff).

## Full lifecycle proven byte-clean on vehu (v pkg only)
`v pkg build` (4 routines + 1 option) → `install --auto-snapshot` → status 3 (XWBPRS
spliced live, `req^VSLRTAP` at CALLP+10; VSLRT* + `^DIC(19,…,25)="run^VSLRTRP"` filed)
→ `uninstall --restore --verify` → **partition** (restore XWBPRS from pre-image,
delete greenfield, ordered), `verifyClean: clean`, `foreignRestore: exact` → vehu
byte-clean (XWBPRS == original, empty diff). Safety guards all held: install without
a snapshot REFUSED (exit 4); double-install REFUSED; bare uninstall with the sidecar
removed REFUSED ("would BRICK the foreign routine").

## Two v-pkg bugs this stress test found AND got fixed (v-pkg@aeadb7e)
1. **SERIOUS** — `install --auto-snapshot` of an already-installed foreign build
   re-captured the post-install routines over the genuine pre-image, destroying the
   back-out. Fixed (snapshot now captured exactly once, gated on not-already-installed).
2. `#19` OPTION routine `run^VSLRTRP` (lowercase tag) failed buildspec validation;
   the validator now accepts lowercase M entry tags.
   Detail: v-pkg memory [[snapshot-clobber-and-lowercase-tag]].

## Still open for P4 (NOT yet built)
The **L12 patch-pin clobber gate** (refuse install when the site's XWBPRS base ≠ the
splice's pinned base) and the **L15 undocumented-seam drift gate** are NOT yet built —
today the splice is regenerated per-site but nothing red-gates a base/patch mismatch.
Next P4 steps (resume prompt): L1–L3 live fault suite, control-verb live smoke, L7
armed load, real GovCloud egress. See the central tracker
`proposals/v-rpc-tap-scalable-implementation-tracker.md`.
