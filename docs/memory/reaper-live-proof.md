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

**CLOSED 2026-06-30 — `v pkg install` ZVPKGRD snag was an m-ydb env gap, now FIXED;
P4 install precondition is HEALTHY.** The `stage ZVPKGRD: read VSLRTAP: driver loaded
no routine` failure was NOT a reaper bug, NOT a v-pkg bug, and NOT a T0a.3 regression:
the **m-ydb driver's source-store (load/sync) path resolved routine write-dirs from the
HOST `$ydb_routines` only, never the container** — so against vehu (no host routines) it
staged nothing, and v-pkg's `runMScript` correctly refused ("driver loaded no routine").
Root-caused + fixed in **m-ydb** (`Session.ContainerRoutines` auto-discovers the
container's own `$ZROUTINES` under docker; `config.SourceStore` falls back to it when host
routines empty — m-ydb commits `7eb90ad`/`ef1ed0e`, memory `m-ydb-docker-gbldir`). **Full
lifecycle now proven byte-clean on vehu:** `v pkg install vslrtap.kids --engine ydb
--transport docker --auto-snapshot` → status 3 / installed:true; `verify` → all 3 present;
`uninstall` → byte-clean (source + `.o` + #9.7 entry + `^XTMP("VPKGI")` gone). So **P4 is
UNBLOCKED** — drive the splice/live steps. GOTCHAS for the live work: a transient
`stage-incomplete: 794 of 345 nodes` was stale `^XTMP("VPKGI")` residue (clean
`VPKGI=0` → 345/345); deleting a stray scratch routine on a VistA engine needs
`^%ZOSF("DEL")` via `m vista exec` (`m-ydb sync rm` leaves the `.o`; `rm` is deny-gated).
The `#19` OPTION shipping in the KIDS build + the splice routine remain the actual P4-step-1
build work. See [[engine-access-through-driver-stack]], [[t0a3-live-install-handoff]].
