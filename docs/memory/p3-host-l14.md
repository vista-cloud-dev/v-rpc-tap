---
name: p3-host-l14
description: The P3 Go host's drain-parse + L14 per-message correlation contract (internal/tap) — why a ring record IS one message (not a req/rsp pair to join), and the two drain wire-format gaps (binary-safety + drop-count not surfaced) that are follow-ups.
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

**Two drain wire-format follow-ups surfaced (NOT yet done — both are small VSLRTH
changes, deferrable):**
1. **Not binary-safe.** The tab-delimited, `\n`-terminated J/V format breaks if a
   captured payload value contains a byte sequence `\nV\t…` or `\nJ\t…` (mis-read as a
   new row). `ParseDrain` joins plain continuation lines (embedded `\n` not followed
   by a row prefix) and keeps embedded TABs (SplitN limit 5), so the common case is
   safe — but the real fix is a **length-prefixed drain** in VSLRTH. Record/result
   *values* don't affect L14 correlation (which keys on inc/job/seq + name + result
   presence), so this is a payload-fidelity hardening, not an L14 blocker.
2. **Drop count not surfaced.** Per-window **drop accounting (R20)** needs the
   trim-drop count, which lives at `^XTMP("VSLRT","buf",job,"drop")` and is **NOT**
   emitted by `drain` (the J header carries job/inc/head/seqmax only). Add `drop` to
   the drain J header (backward-compatible append) so the host can report loss without
   a second status call. Until then `ParseDrain` exposes Head/SeqMax but not drops.

**Command surface logic built** (`internal/host`, also v-pkg-independent): the `Tap`
controller drives VSLRTH (`arm/disarm/status/drain/committrim`) over an `Execer` seam
(`Exec(ctx, command) (string, error)`) — fake-tested, no engine. `Status` parses the
`on=^epoch=^jobs=^records=` line; `Drain` pipes the raw drain through `ParseDrain`+
`Correlate`. The **seam pattern mirrors v-rpc-debug** (`capture.Execer` +
`mdriverExecer` over `mdriver.Client`): the production Execer wraps `mdriver.Client`
(waterline rule 3); the interface exists only so the command logic is testable without
an engine.

Next P3 slices: the real `mdriverExecer` adapter + `engineConn` knobs + the kong CLI
verbs in `rpctapcli` (mirror v-rpc-debug; pulls the clikit/mdriver dep tree) + a live
smoke — the smoke is **gated on `VSLRTH` being on an engine** (the v-pkg install path,
currently the `ZVPKGRD` snag in [[reaper-live-proof]]). Then the read-only `drain → S3
(GovCloud partition) → committrim-after-ack` pipeline (D12) and `validate` vs the native
XWBDEBUG oracle. See the central tracker P3 section.
