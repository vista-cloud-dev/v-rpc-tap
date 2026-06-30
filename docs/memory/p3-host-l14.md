---
name: p3-host-l14
description: The P3 Go host — drain-parse + L14 per-message correlation (internal/tap), the host.Tap controller (internal/host), and the rpctapcli CLI verbs (arm/disarm/status/drain/committrim) — why a ring record IS one message (not a req/rsp pair to join), the verb Run/exec split for engine-free testing, the VRPCTAP_ env prefix, and the two drain wire-format follow-ups (binary-safety + drop-count).
metadata:
  type: project
---

The first P3 host slice is `internal/tap` (pure Go, no engine): `ParseDrain` turns a
VSLRTH drain stream into `Record`s, and `Correlate`/`Summarize` implement the **L14
per-[XWB]-message correlation contract** — table-driven tested, gate-green
(coverage 88%).

**Key insight — one ring record IS one [XWB] message; L14 is classification, not
req↔resp pairing.** The proposal's "correlate req↔resp by call_id" framing is
misleading. The in-path splice fires at req@15.5 (writes the base record + params at
seq N) and again at rsp@17.5 (adds the **result** to the **SAME** seq N — `workR`
reads the current seq). So req and rsp are **not two records to join** — they are one
ring node that rsp augments. The host's job is therefore:
- **classify** each record: `complete` (mode 2 with a result) / `unpaired` (mode 2,
  no result — denied/errored/in-flight) / `names-only` (mode 1, req-only by design);
  `empty-name` (`RPC==""`) is an orthogonal flag, the only "noise" that reaches the
  tap (TCPConnect/#BYE# never hit req@15.5, per P0);
- **key** every record by the durable `(inc, job, seq)` triple (`Record.Key()`);
- **segment** by `(inc, job)` into sessions — a reused `$J` gets a fresh `inc`, so
  two incarnations never conflate;
- **de-dupe** re-drained records (at-least-once delivery) on Key, **preferring the
  copy that carries a result** (a later drain caught rsp). `tolerate` is the contract:
  unpaired/empty-name records are kept + counted, never dropped or errored.

**Two drain wire-format follow-ups — DONE (2026-06-30, dual-engine + Go gate green):**
1. **Binary-safe via a length-prefixed value line.** The J/V header lines stay
   TAB-framed (they hold only controlled tokens — integers, the inc token, a `$C(2)`-
   joined subpath of controlled subscripts), but the one arbitrary field — the captured
   value — moved to its **own length-prefixed line**: `V…\t<bytelen>\n<value bytes>\n`.
   `ParseDrain` reads exactly `bytelen` bytes (`io.ReadFull` over a `bufio.Reader`, not a
   line `Scanner`), so a value containing TAB/`\n` or even a forged `\nV\t…` row prefix
   can **never** be mis-split (the old continuation-join heuristic is gone). On the M
   side the line construction is now pure functions **`$$jhdr(job,head,sm)` / `$$vhdr(job,
   seq,sp,v)`** (`vhdr` emits `$length(v)`) — `drain`/`dumpNode` just `write $$…,!` then
   `write v,!`. That refactor is what makes the wire format **unit-testable in M without
   device capture** (assert the returned line's pieces); device-capture in the STDASSERT
   harness is impractical.
2. **Drop count surfaced in the J header.** `$$jhdr` appends `\t<drop>` (the per-job
   cumulative `^XTMP("VSLRT","buf",job,"drop")`) as a 6th field — a **backward-compatible
   append** (`ParseDrain` tolerates a 5-field header, drop→0). `Record.Drop` carries it;
   `Summarize` adds `Summary.Dropped` = the drop summed **once per distinct job** (every
   record of a job repeats the same header value, so summing per-record would inflate it).
   The `drain` verb now reports per-window loss with no second status call (R20).
   *Limitation:* a job with drops but zero live records emits a J line with no V lines, so
   no `Record` carries its drop — that count is in the stream but not in `Summary.Dropped`
   (rare; a fuller fix would return per-job headers from `ParseDrain`).

**Command surface logic built** (`internal/host`, also v-pkg-independent): the `Tap`
controller drives VSLRTH (`arm/disarm/status/drain/committrim`) over an `Execer` seam
(`Exec(ctx, command) (string, error)`) — fake-tested, no engine. `Status` parses the
`on=^epoch=^jobs=^records=` line; `Drain` pipes the raw drain through `ParseDrain`+
`Correlate`. The **seam pattern mirrors v-rpc-debug** (`capture.Execer` +
`mdriverExecer` over `mdriver.Client`): the production Execer wraps `mdriver.Client`
(waterline rule 3); the interface exists only so the command logic is testable without
an engine.

**P3 CLI wiring DONE** (`rpctapcli`, dep-wired + table-tested, gate-green): the real
`mdriverExecer` (adapts `mdriver.Client.ExecEval` → `host.Execer`; an `EngineError`
becomes a Go error) + `engineConn` knobs + the kong `Control` verbs **arm / disarm /
status / drain / committrim** mounted on `Commands`, each delegating to the `host.Tap`
controller. Durable points:
- **Each verb splits `Run` (resolves the Execer over the driver seam) from `exec(cc, *host.Tap)`
  (the testable logic).** The shared `run()` prologue does `execer()→host.New(ex)→exec`; tests
  drive `exec` with a fake Execer (no engine), so command-string generation + result shaping are
  verified bare. Build a `*clikit.Context{Stdout, Stderr, Format}` literal in tests — Result's
  JSON/text paths need no theme.
- **Env prefix is `VRPCTAP_*`, NOT v-rpc-debug's `VRPC_*`** — deliberately distinct so the two
  domains don't collide on process-global env when both are mounted under one `v` umbrella process.
- **v-rpc-tap imports the SHARED `clikit` v0.7.0 + `m-driver-sdk` v0.3.0** — it is a generic `v`
  CLI consumer, NOT a driver, so it correctly takes the shared clikit module (do NOT "fix" it to
  the contract-bearing driver fork — see [[driver-clikit-fork]]). `make go-build` resolves via the
  root go.work; go.mod pins the real versions for the `GOWORK=off` CI build.
- **Live smoke DEFERRED** (gated on `VSLRTH` being installed on an engine = the v-pkg install path,
  the `ZVPKGRD` snag in [[reaper-live-proof]]); the uncovered `rpctapcli` lines are exactly those
  engine-bound `Run`/`execer` paths.

Remaining P3 slices: the live smoke (when v-pkg install is healthy); the read-only `drain → S3
(GovCloud partition) → committrim-after-ack` pipeline (D12); `validate` vs the native XWBDEBUG
oracle. (Drain-format hardening — binary-safe length-prefix + drop-count — is now DONE, above.)
See the central tracker P3 section.
