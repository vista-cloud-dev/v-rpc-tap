# Memory index — v-rpc-tap (per-repo)

Per-repo durable memory for the `v-rpc-tap` repo (the hybrid Go+M `v` domain that
ships the **VSL RPC TAP** M package — the in-path RPC-broker tap — plus its Go
host). The **shared** cross-cutting tap proposal/tracker/coordination memory lives
in the central `docs` repo (`proposals/v-rpc-tap-scalable*`, `memory/v-rpc-tap-scalable.md`);
this file holds only facts durable to THIS repo's code. One line per entry — detail
in the linked topic file. Keep-test applies (durable lessons only, not per-increment status).

- [P2 L-block safety harness](p2-lblock-safety-harness.md) — in-path fences (fail-open trap, naked-ref fence, zero-I/O) proven under fault by `tests/VSLRTLTST.m` (L1–L3) + ring multi-actor invariants (single trim owner D8, segmentation D13, durability watermark D12) proven by `tests/VSLRTCTST.m` (L4–L6). Class-agnostic trap; the **deterministic-interleaving** proof technique (sound: no shared written node + atomic `$INCREMENT`); the **inc-token same-second/same-DUZ collision** gotcha + reaper-is-load-bearing-for-segmentation; the **STDASSERT `$X`/`$Y`** + simulate-an-append-fully test gotchas. 5000-load = deferred L7.
