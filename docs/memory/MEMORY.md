# Memory index — v-rpc-tap (per-repo)

Per-repo durable memory for the `v-rpc-tap` repo (the hybrid Go+M `v` domain that
ships the **VSL RPC TAP** M package — the in-path RPC-broker tap — plus its Go
host). The **shared** cross-cutting tap proposal/tracker/coordination memory lives
in the central `docs` repo (`proposals/v-rpc-tap-scalable*`, `memory/v-rpc-tap-scalable.md`);
this file holds only facts durable to THIS repo's code. One line per entry — detail
in the linked topic file. Keep-test applies (durable lessons only, not per-increment status).

- [P2 L-block safety harness](p2-lblock-safety-harness.md) — the in-path fences (fail-open trap, naked-ref fence, zero-I/O) are proven under fault by `tests/VSLRTLTST.m` at the bare-engine unit level; what L1–L3 cover, why the fail-open trap is class-agnostic, and what's deferred to the live-broker / 5000-load phases. Includes the **STDASSERT `$X`/`$Y` testing gotcha**.
