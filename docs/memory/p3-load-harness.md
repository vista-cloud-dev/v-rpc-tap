---
name: p3-load-harness
description: The L8 synthetic-[XWB] load-harness rig (internal/xwbwire + internal/load + the `load` verb) — why it is a broker TCP client (NOT the engine seam), how it instruments throughput/latency, and why the armed measurement waits on the splice while the rig itself is independent.
metadata:
  type: project
---

The **L8 load-harness rig (R15)** is built and gate-green (pure Go, fake-broker-tested):
- **`internal/xwbwire`** — the `[XWB]` wire encoder (`RPCMessage`), seeded byte-for-byte
  from v-rpc-debug's proven client; cover 100%.
- **`internal/load`** — `Run(ctx, Config)` drives `Concurrency` workers firing a workload
  `Mix` of RPCs (fresh connection per RPC, as the broker may end the job after one
  unauthenticated message), in either **Total** mode (shared atomic budget) or **Duration**
  mode (context deadline). `summarize` reduces samples to throughput + **nearest-rank**
  p50/p95/p99/min/mean/max over the *successful* calls; failures are counted, never silently
  dropped. `Delta(control, armed)` computes the armed-vs-control latency overhead + throughput
  drop %. cover 95%.
- **`load` verb** (`rpctapcli`, group Bench) — `--addr/--concurrency/--total/--duration`;
  `--duration` overrides the default `--total`.

**Key design point — the rig is an RPC *client* of the broker TCP socket, NOT the engine
seam.** It talks `[XWB]` straight to `--addr` (the CPRS-equivalent), so it takes a broker
host:port, *not* the m-driver/`engineConn` flags — the same legitimate exception
v-rpc-debug's `ping` uses. The m/v waterline transport-monopoly (rule 3) governs reaching
the M *engine* to actuate it; firing wire-protocol traffic at the broker port as a load
generator is not that. So no `mdriver.Client` here, and it's correctly waterline-clean.

**What's independent vs gated:** the rig + the **control run** (load against an *unspliced*
broker) are buildable/runnable now. The **armed measurement** — the same run with the VSL
RPC TAP spliced into dispatch, to read the latency tax off `Delta()` — waits on the live
splice (the v-pkg install path). The full **5000-session live knee (L7)** also waits on the
splice + a sized host; this increment delivers only the *mechanism* (L8), tested against an
in-process fake broker over real loopback sockets. See [[p3-host-l14]] for the rest of the
host and the central tracker L7/L8/L9 rows.
