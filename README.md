# v-rpc-tap

The **`v rpc-tap`** domain of the `v` CLI — VistA's **durable, scalable RPC
tap**: an `XWBPRS` splice that streams full-payload RPC traffic through a
per-job bounded ring and drains it read-only to S3 for high-volume capture.

This is the **high-risk** half of the RPC-tap split. It ships both the
**`VSL RPC TAP` M package** (the `VSLRT*` routines + KIDS build that splice the
broker) and the **host tooling** (arm / drain → S3 / validate) plus the
dual-engine + 5000-user **load-proof harness**. It modifies a national,
Class-1-adjacent routine, so it is built and proven **in full on our own
gold-master engines** (`vehu` YDB, `foia*` IRIS, through the driver stack)
before it is ever offered for VA implementation.

## Relationship to v-rpc-debug

`v-rpc-tap` is the deliberate sibling of **[`v-rpc-debug`](https://github.com/vista-cloud-dev/v-rpc-debug)**,
the *safe, read-only* XWBDEBUG oracle. The two are kept in **separate repos**
with opposite risk profiles and integrate **only at the v-cli busybox**, where
they surface as two sibling domains — `v rpc-debug` and `v rpc-tap` — never as a
merged `v rpc` node and never in one codebase. v-rpc-debug's capture is the
offline correctness oracle this tap is validated against.

## Status

**Design-complete, not yet built.** Baseline = proposal v3.6.0 (twice
fact-checked against live broker source + the vdocs GOLD corpus + v-pkg Go).
The next increment is P0 (pin the seam on both engines) and the P1 KIDS build,
with the dual-engine + 5000-user load proof (L1–L11) as the headline
obligation. See [`docs/README.md`](docs/README.md) for the proposal, tracker,
and analyses.

## Layout

This is a **hybrid Go+M `v` repo** — the first in the org. The M routines + KIDS
build live here (not in `v-stdlib`) by deliberate decision (proposal D2): the tap
**mutates a national, checksum-audited routine** (`XWBPRS`), so it does **not**
belong in the additive-only "VistA Standard Library." The M apparatus mirrors
`v-stdlib`; the Go host mirrors `v-rpc-debug`.

```
src/         VSL RPC TAP M routines (VSLRTAP/VSLRTRP/VSLRTH)  — P1a
tests/       M unit tests (VSLRT*TST.m, STDASSERT)            — P1a
kids/        VSL RPC TAP KIDS build spec (vslrtap.build.json) — P1a
.m-cli.toml  M lint/fmt config (modern, dual-engine)          — P1a
rpctapcli/   importable Go command surface (mounted `v rpc-tap` — empty until P3)
docs/        proposal pointers, tracker, memory
```

M side: `m test`/`m coverage`/`m lint` (engine work through the driver stack only).
Go side: `go test ./...`. The root `Makefile` runs both.

License: AGPL-3.0 (see `LICENSE`).
