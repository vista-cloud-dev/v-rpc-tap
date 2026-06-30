---
name: p2-lblock-safety-harness
description: How VSLRTAP's in-path fences are proven under fault (P2 L1-L3), the class-agnostic-trap argument, the deferred halves, and the STDASSERT $X/$Y testing gotcha.
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
