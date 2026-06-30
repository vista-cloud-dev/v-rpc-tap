---
name: reaper-live-proof
description: How VSLRTRP's live-engine-only behaviours (L13 liveness token + REQ^%ZTLOAD requeue) were proven on the gold masters vehu/foia by surgical inline `m vista exec`, the Kernel TaskMan cleanup gotchas, and the open v-pkg-install staging snag.
metadata:
  type: project
---

The reaper's two **live-engine-only** behaviours — the ones the bare test engines
(m-test-engine / m-test-iris) cannot exercise because they lack Kernel TaskMan and a
real PID space — are proven on BOTH gold masters (`vehu` YDB-VistA, `foia-t12`
IRIS-VistA) through the driver stack (`m vista exec --transport docker`), 2026-06-30:

- **L13 reaper-liveness token (U3/R24) live.** The exact runtime-detect XECUTE idiom
  `orphans^VSLRTRP` uses resolves to the correct per-engine primitive on each LIVE
  engine and returns 1 for the live `$J` / 0 for a dead PID: **YDB/vehu** →
  `$ZGETJPI($J,"ISPROCALIVE")`; **IRIS/foia** → `''$data(^$JOB($J))`. Confirms the
  per-engine split (the branch that *errors* on the other engine) works on the live
  engines, not just the bare ones.
- **Live `REQ^%ZTLOAD` requeue.** The reaper's exact `ZT*` shape
  (`ZTRTN="run^VSLRTRP"`, `ZTDTH`, `ZTIO=""`) queues a real TaskMan task on both
  (`do ^%ZTLOAD` returned a task number on vehu and foia) — proving
  `requeue()`/`start()`'s body works on live VistA TaskMan.

**Method — prove it INLINE, no install needed.** The live claims are engine-specific
code *fragments*, so they were proven by executing the exact fragments via
`m vista exec` (the sanctioned ad-hoc driver-stack path) rather than installing the
KIDS build. This needs no `v pkg install`, leaves both gold masters pristine (only a
transient TaskMan task, immediately removed), and is the faithful proof of the
behaviours — "a filed routine runs under TaskMan" is a separate, already-proven
(T0a.3) KIDS-install concern, not a reaper-logic one.

**GOTCHA — Kernel TaskMan task cleanup.** `DEQ^%ZTLOAD` does **NOT exist**
(`$text(DEQ^%ZTLOAD)=""`); the supported removal API is **`KILL^%ZTLOAD`**
(`S ZTSK=<n> D KILL^%ZTLOAD`), with `ISQED^%ZTLOAD` to test queued-state. A task
created by `do ^%ZTLOAD` **persists in `^%ZTSK(<n>)` across `m vista exec` processes**
(separate jobs, shared globals) — so any test task MUST be cleaned up, or it fires
later and errors (the routine may not exist on the engine). Always schedule proof
tasks `ZTDTH` far out (e.g. `$H` day+1) so they can't fire before you `KILL` them.
Verify removal with `$d(^%ZTSK(<n>))=0`. The task's routine sits in
`$piece(^%ZTSK(<n>,0),"^",1)` (note: that value `run^VSLRTRP` itself contains a `^`,
so a naive `$piece(...,"^",1)` yields just `run` — fine as a recent-task fingerprint).

**GOTCHA — `m vista exec` swallows FOR-loop output.** A command whose M contains a
`FOR` loop returns **empty stdout** (the device buffer is lost). Probe with single
statements / unrolled per-item checks, never a scan loop. (Same family as the
argumentless-FOR-swallows-WRITE note in [[kernel-vsl-coverage-audit]].)

**GOTCHA — m-iris driver not auto-discovered.** `m vista exec --engine iris` errors
`NO_DRIVER` unless **`M_IRIS_BIN=…/m-iris/dist/m-iris`** is exported (m-ydb auto-resolves
from `../m-ydb/dist/`; m-iris did not from `v-rpc-tap` cwd). Container env mirrors vehu:
`M_IRIS_CONTAINER=foia-t12 … --transport docker`.

**OPEN — `v pkg install` staging snag on the gold masters.** `v pkg install
dist/kids/vslrtap.kids --engine ydb --transport docker` (M_YDB_CONTAINER=vehu) fails at
**stage `ZVPKGRD`**: `read VSLRTAP: driver loaded no routine (check the engine's routine
source path / connection)` — the install's scratch-routine staging/read-back, NOT the
reaper logic. `m vista exec` (eval) works on the same connection, so it is specific to
routine *staging* via the docker transport against a VistA image. T0a.3 had the full
install/uninstall lifecycle working on vehu, so this looks like a regression or an env
gap (routine-write path) — needs a focused v-pkg/driver session. The inline proof above
deliberately bypassed it; the additive install + shipping the `#19` OPTION in the KIDS
build remain owed for the full live reaper-live close. See [[engine-access-through-driver-stack]],
[[t0a3-live-install-handoff]] (shared docs memory), and the P2 reaper rows in the
central tracker.
