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

## How L1-L3 live will use it (next increment)
Install splice → `arm` the tap → `drive --rpc …` → `drain` ring confirms `req^VSLRTAP`
fired inside the REAL broker job (= nested in the live `$ETRAP="D ETRAP^XWBTCPM(0)"`,
`XWBTCPM:158`). Non-interference = response bytes identical armed vs disarmed. L2 fault
injection needs typed malformed param chunks via `FireRaw` (build `RPCWithParams` then).
See the central tracker `proposals/v-rpc-tap-scalable-implementation-tracker.md` L1-L3.
