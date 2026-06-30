---
name: p4-handshake-client
description: Driving real RPCs through the installed splice REQUIRES a TCPConnect handshake first — a bare RPC chunk is rejected at NEW^XWBTCPM before MAIN/CALLP, so the tap never fires. The handshake-capable broker client (internal/xwbwire + internal/broker + the `drive` verb), and the latent L8 load-rig defect this exposed.
metadata:
  type: project
---

**To reach the splice you MUST complete the broker handshake first.** This is a durable
invariant of the RPC Broker, not a quirk — and it invalidated the resume prompt's
assumption that the L8 load rig could drive the armed splice.

## Why a bare RPC chunk never reaches the tap
The tap splices `req^/rsp^VSLRTAP` into `CALLP^XWBPRS`, which is called ONLY from the
`MAIN` loop (`XWBTCPM:187`). Reaching `MAIN` requires an accepted connection:
- `CONNTYPE` reads the FIRST message; `[XWB]` → `NEW`.
- `NEW` parses that first message inline via `$$PRSM^XWBPRS` (NOT `CALLP`), builds `MSG`
  from the command chunk, and **rejects + quits unless the command is `TCPConnect`**
  (`XWBTCPM:137`, `QSND "reject"` then `Q`).
- Only an accepted handshake falls through to `RESTART`→`MAIN`, whose loop calls
  `D CALLP^XWBPRS` per subsequent message.

So a client that opens a socket and writes a single bare RPC chunk (the old
`xwbwire.RPCMessage`, and the L8 `load.fire()`) is rejected at the handshake; its RPC
name is logged by the pre-`MAIN` parse (which is exactly what the 2026-06-26 XWBDEBUG
host-probe saw — `RPC:` at `PRS2`), but `req^VSLRTAP` never fires.

**Latent L8 defect this exposed:** the tracker's "L8 rig built; armed run waits on the
splice" was wrong — even with the splice installed, the handshake-less rig captures
nothing. The fix (a real handshake) is shared with this client and also unblocks L7/L8.

## The handshake-capable client (built 2026-06-30, TDD, go-check green)
Wire bytes are reproduced VERBATIM from `XWBTCPMT.m` (the VA-shipped broker
connection-test routine, `TEST(IP,PORT)`), so they are known-good, not hand-derived:
- `internal/xwbwire` (pure, 100% cover): `Connect()` (= XWBTCPMT:31 TCPConnect),
  `RPCMessage(name)` (= :34 form: header `1130` + RPC chunk + the **required** `54f`
  empty param chunk — without a type-5 chunk `XWB("PARAM")` is unset and `CALLP`'s
  `D CAPI(.XWBP,XWB("PARAM"))` would UNDEF in the broker), `Bye()` (= :37 `#BYE#`).
- `internal/broker` (Session I/O): `Dial` (TCP + handshake; errors if the broker replies
  anything but `accept`), `Fire(name)`/`FireRaw(bytes)`, `Close` (sends `#BYE#`).
  Reads until the **`0x04` EOT** that ends every response (`SND^XWBRW` →
  `WRITE($C(4))`). Accept=`\0\0accept\x04`, reject=`\0\0reject\x04`.
- `rpctapcli` `drive` verb (group Bench): one session, fires `--rpc NAME…` in order,
  reports each response's exact length + hex → control-vs-armed byte diff for L1-L3
  non-interference (CF4). A broker TCP client (`--addr`), not the engine seam.

## L1-L3 PROVEN LIVE on vehu (YDB), 2026-06-30 — recipe + the method split
Full cycle, gold master left byte-clean (`v pkg` only): build kids from the site's
current XWBPRS (`sync diff` empty vs static → valid §12 input) → `v pkg install
--auto-snapshot` (status 3, splice live) → `arm --mode 2` → `drive --rpc …` (3 RPCs) →
`status` records=4 / `drain` (complete=2, unpaired=2, emptyName=1) → `v pkg uninstall
--restore --verify` (partition, `verifyClean:clean`, `foreignRestore:exact`) → `K
^XTMP("VSLRT")` → `sync diff` empty + routines gone.

- **L1 capture (wire-driven):** driving real RPCs via the handshake `Session` made
  `req^/rsp^VSLRTAP` fire inside the REAL broker job — 4 ring records from 3 RPCs.
- **Non-interference (wire-driven, byte-exact):** the `drive` verb's response hex is
  **identical armed vs unspliced control** for every RPC (XWB IM HERE 4B, XUS INTRO MSG
  1780B, XWB GET VARIABLE VALUE 90B) → the tap adds ZERO wire bytes (CF4). This is the
  real-socket proof Option B promised.

**METHOD SPLIT — durable lesson: you CANNOT force a tap-specific fault through the wire.**
The broker walks the same params (PRS5 LINST/GINST) *before* the tap, so a malformed
param faults the broker, not the tap. So the L1-fault/L2/L3 **fault path** is proven
**in-engine against the REAL installed routine** via `m vista exec` (the sanctioned
driver path), replicating `VSLRTLTST`'s injections with a sentinel outer `$ETRAP` and a
`$X/$Y` delta as the CF4 proxy:
- req fault (`XWB(5,"P",1)="^"` → capParams UNDEF/NAKED): `bt=0` (outer broker trap never
  fired), `sent=1` (control returns), `on=0` (self-disabled), `$EC` clean, `dx=dy=0`.
- rsp fault (`XWBPTYPE=4,XWBP="^"` → capResult gwalk): same — `bt=0 on=0 dx=dy=0`.
- L3 naked-ref (`^ZZRT("b",1)` indicator across a faulting req): `^(1)=22` restored.

Fidelity honestly stated: non-interference + capture = real socket through the live
broker; fault path = real installed routine on the real engine under a sentinel outer
trap (not socket-driven — a socket-driven tap fault isn't achievable without a fault
hook in the routine, which we will not ship). **OWED: mirror the whole cycle on foia
(IRIS)** — `--engine iris`, broker host `19430`, `M_IRIS_*` creds.
