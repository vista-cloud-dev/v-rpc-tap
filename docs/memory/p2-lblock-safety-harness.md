---
name: p2-lblock-safety-harness
description: How the P2 L-block proves VSLRTAP's in-path fences under fault (L1-L3, VSLRTLTST) and the ring's multi-actor invariants (L4-L6, VSLRTCTST) — the class-agnostic-trap argument, the deterministic-interleaving proof technique, the inc-token collision gotcha, and the STDASSERT $X/$Y gotcha.
metadata:
  type: project
---

`tests/VSLRTLTST.m` is the P2 L-block in-path **safety** suite. It proves the fences
already built into `VSLRTAP` (P1a) under deliberate fault, dual-engine (YDB
m-test-engine + IRIS m-test-iris) through the driver stack. No new production code —
a verification harness; the fences pre-exist, the suite proves they hold.

**What it proves (L1-L3, single-user, bare engine — no 5000-rig):**
- **L1** trap absorbed, not propagated: a fault inside capture is caught by VSLRTAP's
  own `$ETRAP="D fail^VSLRTAP"` and resumes in `req()`/`rsp()`; an OUTER
  (broker-emulating) `$ETRAP` set by the test **never fires**, control returns
  normally, and the broker-contract vars (`XWB`/`XWBP`/`XWBPTYPE`) are byte-identical.
- **L2** forced fault → self-disable (`^XTMP("VSLRT","ON")` killed, `$ECODE` clean) +
  **zero device I/O** ($X/$Y do not move across req/rsp), in BOTH the req and rsp paths.
- **L3** naked-reference integrity across req including the **fault-resume** path
  (a fault mid-capture still re-references the saved naked ref at req/rsp step 3).

**Why two malformed-reference faults cover every named class.** The fail-open trap is
**class-agnostic** — one `$ETRAP` wraps the whole risky body, so the disable +
zero-I/O behaviour does not depend on the error class. The suite injects two
*deterministic, dual-engine* faults via the capture inputs (no source edits):
`XWB(5,"P",1)="^"` (bad global referent → UNDEF/NAKED in `gwalk`) and
`XWB(5,"P",1)=".XWBS1("` (malformed list descriptor → SYNTAX/SUBSCRIPT in the `@ref`
indirection). MAXSTRING is **not** a portable in-path trigger (YDB max ~1 MB vs IRIS
~3.6 MB, and the capture body only SETs values it reads — it never concatenates beyond
`rec()`'s small pieces); the I/O class is **structurally absent** (the in-path body
contains no WRITE/READ/OPEN/USE — that absence is the very invariant L2 asserts). Both
are documented in the suite header; a natural MAXSTRING trigger (a giant result) lands
with the live-broker phase.

**Deferred to later P2 increments (NOT in this suite):** the *live-broker-nested* half
(install the real `XWBPRS` splice via `v pkg install` and drive real RPCs so our trap
nests inside the broker's actual `$ETRAP`), and the 5000-user load proof (L7-L11) per
the BB2 topology. L1-L3 here are the unit-level fence proofs; they are the highest-value
correctness gates and need no rig.

**GOTCHA — STDASSERT output moves `$X`/`$Y`.** A test that asserts a `$X`/`$Y` delta
(the zero-I/O proxy) MUST snapshot `$X`/`$Y` into locals *immediately after* the call
under test and assert against those locals — because `eq^STDASSERT` writes a PASS line
to the device, which advances `$X`/`$Y`. Reading `$y` live in a *second* assert (after
the first assert already printed) measures the harness, not the tap, and fails
spuriously. This bit hard: every assertion passed in isolation but two failed when run
in sequence. Pattern:
`set x0=$x,y0=$y  do call()  set x1=$x,y1=$y  do eq(...,x1,x0,...)  do eq(...,y1,y0,...)`.

Engine access for this suite is the bare test engines through the m driver stack only
(`m test --engine ydb --docker m-test-engine` / `--engine iris --docker m-test-iris`);
see [[engine-access-through-driver-stack]] (shared docs memory). Tracker: the central
`docs` repo `proposals/v-rpc-tap-scalable-implementation-tracker.md` P2 / L-block.

---

## L4-L6 — ring concurrency + durability invariants (`tests/VSLRTCTST.m`)

The second P2 suite proves the ring's **multi-actor invariants** — single trim owner
(D8), per-incarnation segmentation (D13), durability watermark / at-least-once (D12/F-E) —
that must hold when the in-path writer (`trim^VSLRTAP`), the off-path drainer +
`committrim^VSLRTH`, and the reaper (`orphans^VSLRTRP`) all touch one
`^XTMP("VSLRT","buf",$J,*)` ring. Like L1-L3 it adds **no production code** — the fences
pre-exist; the suite proves they compose. Dual-engine 33/33.

**Durable technique — prove ring concurrency in a SINGLE M job by deterministic
interleaving.** The m-test harness is one process, so true concurrency is impossible here,
yet the invariants are still fully provable: drive each actor by hand in every order the
real race window can produce and assert head/wm/seq. This is **sound because the design has
no shared written node across writers** (each `$J` owns its ring; `$INCREMENT` is atomic) —
so the only multi-actor surface is a single ring's head/wm/seq, whose orderings are finite
and enumerable. What it does NOT cover (be honest in the suite header, as L1-L3 do for
MAXSTRING/I-O): throughput and the fill-vs-drain knee under 5000 real processes — that is
the **L7 load proof**, a different obligation. Reuse this technique for any future
ring-invariant proof before reaching for a real-concurrency rig.

**The single-owner invariant that L4 nails (D8/D12):** a record is removed by **exactly
one** actor — in-path `trim` (drop-oldest, only past the depth cap **and** only below the
drained watermark `wm`) **or** `committrim` (delete the acked prefix, post-PUT) — never
both, never neither-silently. `trim` refusing to cross `wm` is what makes a slow/crashed
drainer safe (records pile up and the reaper's `overflow` disarms, rather than losing
undrained data). `committrim` advancing head past `wm` is what lets `trim` reclaim the
acked prefix. Head is monotonic across both owners.

**GOTCHA — segmentation depends on the reaper, and the inc token can collide.** D13's
per-incarnation token is `^XTMP("VSLRT","buf",$J,"inc")=$horolog_"-"_$get(DUZ)`, stamped
once at `seq=1`. Two truths the L5 tests pin: (1) **the orphan reaper is the load-bearing
segmentation mechanism** — a recycled `$J` only gets a fresh `inc` + seq-restart because the
reaper KILLed the dead incarnation's ring first; reuse *before* a reap keeps the old `inc`
and conflates the two sessions (proven, by design). (2) The token is **second-granularity +
DUZ**, so a PID reused **within the same second by the same DUZ** across a reap would
collide. Realistic reuse is a *different* user's signon (distinct DUZ → distinct token), so
the window is narrow, but it is a real D13 edge — a sub-second/​counter component
(YDB `$ZUT` µs vs IRIS fractional `$ZHOROLOG` — needs the same per-engine-token treatment as
the R12 naked-ref SVN) would close it. Left as an **owner call**, recorded in the tracker
L4-L6 row; do NOT silently "fix" it (cross-engine portability + it touches the wire schema).

**Test-authoring gotcha (cost one red iteration):** when a test bumps `…,"seq"` to simulate
a fresh in-path append, it must **also SET the record node** at that seq — otherwise a
later `present(j,lo,hi)` helper fails on the missing node, which looks like a production bug
but is a test omission. Simulate an append fully (seq counter **and** the `buf,$J,seq` value)
or drive `req^VSLRTAP()` for real.
