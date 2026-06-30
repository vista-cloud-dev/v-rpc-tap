---
name: p3-load-harness
description: The L8 synthetic-[XWB] load-harness rig (internal/xwbwire + internal/load + the `load` verb) — why it is a broker TCP client (NOT the engine seam), how it instruments throughput/latency, and why the armed measurement waits on the splice while the rig itself is independent.
metadata:
  type: project
---

The **L8 load-harness rig (R15)** is built and gate-green (pure Go, fake-broker-tested):
- **`internal/xwbwire`** — the `[XWB]` wire encoder; cover 100%. See [[p4-handshake-client]].
- **`internal/load`** — `Run(ctx, Config)` drives `Concurrency` workers, in either **Total**
  mode (shared atomic budget) or **Duration** mode (context deadline). `summarize` reduces
  samples to throughput + **nearest-rank** p50/p95/p99/min/mean/max over the *successful*
  calls; failures are counted, never silently dropped. `Delta(control, armed)` computes the
  armed-vs-control latency overhead + throughput drop %. cover 95%.
- **`load` verb** (`rpctapcli`, group Bench) — `--addr/--concurrency/--total/--duration`;
  `--duration` overrides the default `--total`.

## ⚠️ REFIT 2026-06-30 — each worker now holds a persistent handshake `broker.Session`
The original `fire()` opened a fresh connection per RPC and wrote a **bare RPC chunk** — which
is **rejected at `NEW^XWBTCPM:137` before `MAIN`/`CALLP`**, so the armed run would have captured
**NOTHING** (the L8 defect, see [[p4-handshake-client]]). Refit: each worker does
`broker.Dial` (TCPConnect handshake) ONCE, fires its claimed RPCs through that session (each
`Fire` reaches `CALLP^XWBPRS` — the splice point), and `#BYE#`s on close. This also models a
real CPRS client (one session, many RPCs). A failed handshake / broken session makes every
**subsequently-claimed** budget unit a failure (no silent drops — preserves the dead-endpoint
contract). **PROVEN on vehu (live):** control run 200 sent / 0 failed; heartbeat-only **4368
rpc/s** (p50 0.099ms, p99 1.2ms); **armed run captured 110 records across 10 jobs** (the defect
is fixed — armed run drives the tap). Gold master left byte-clean.

**L7 workload finding (not a rig bug):** the `defaultMix` RPCs `XUS INTRO MSG` / `XWB GET
VARIABLE VALUE` show ~2s p99 outliers **under concurrency on the no-session path** (the broker
waits for follow-up input), while `XWB IM HERE` is clean at 4368 rpc/s. So L7 must use a
clean fast workload (or a real signed-on session), not the unauthenticated default mix —
pick the mix deliberately when characterizing the latency tax.

**Key design point — the rig is an RPC *client* of the broker TCP socket, NOT the engine
seam.** It talks `[XWB]` straight to `--addr` (the CPRS-equivalent), so it takes a broker
host:port, *not* the m-driver/`engineConn` flags — the same legitimate exception
v-rpc-debug's `ping` uses. The m/v waterline transport-monopoly (rule 3) governs reaching
the M *engine* to actuate it; firing wire-protocol traffic at the broker port as a load
generator is not that. So no `mdriver.Client` here, and it's correctly waterline-clean.

**What's done vs gated:** the rig, the **control run**, and the **armed run** all work now
(both proven on vehu, above). The full **5000-session live knee (L7)** still waits on a sized
host (raised `ulimit`/`gtm_procs`/buffers per BB2) + a deliberate clean workload + the
single-vs-sharded-drain decision. See [[p3-host-l14]] for the rest of the host, [[p4-handshake-client]]
for the session client, and the central tracker L7/L8/L9 rows.
