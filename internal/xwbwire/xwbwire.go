// Package xwbwire encodes RPC Broker "[XWB]" wire-protocol messages — enough for a
// client to complete the TCPConnect handshake and then fire RPCs at a broker, so the
// load harness and the live L-block tests have real traffic to drive. It is pure byte
// assembly (no I/O), so it is unit-testable; the TCP send lives in internal/broker.
//
// This is the v rpc-tap domain acting as an RPC *client* (like CPRS), talking the
// broker's TCP wire protocol to generate load — distinct from reaching the M engine,
// which always goes through the m-driver-sdk seam.
//
// WHY A HANDSHAKE: a broker connection's first message must carry the "TCPConnect"
// command or NEW^XWBTCPM rejects it and the handler quits (XWBTCPM:137) BEFORE the
// MAIN loop. The tap is spliced into CALLP^XWBPRS, which MAIN calls (:187) only on an
// accepted connection. So a bare RPC chunk never reaches the tap — the client must
// send Connect() first, read the "accept", THEN fire RPCMessage()s, then Bye().
//
// The byte sequences are reproduced verbatim from XWBTCPMT.m (the VA-shipped broker
// connection-test routine), so they are known-good rather than hand-derived.
//
// Message (client->server): "[XWB]" + 4 header bytes + chunks + 0x04 (EOT). A chunk is
// a 1-byte type then its body. SREAD = one length byte (<=255) + value.
package xwbwire

// sread encodes a value as one length byte followed by the bytes (len <= 255).
func sread(s string) []byte {
	b := []byte(s)
	return append([]byte{byte(len(b))}, b...)
}

// connectParams is the type-5 param chunk of XWBTCPMT:31 (IP/socket/manager), verbatim
// — it starts with the '5' chunk type. These values are cosmetic to the server (logged
// + used for license/peer), so the known-good literal is reproduced rather than rebuilt.
const connectCmd = "TCPConnect"
const connectParams = "50010127.0.0.1f00010f0024ISF-FORTW.vha.domain.extf"

// Connect builds the TCPConnect handshake message. The server accepts the connection
// (QSND "accept") and proceeds to MAIN; the client must send this first.
func Connect() []byte {
	msg := []byte("[XWB]1030")              // marker + header (VER=1,TYPE=0,LENV=3,RT=0)
	msg = append(msg, '4')                  // chunk type 4 = Command
	msg = append(msg, sread(connectCmd)...) // 0x0a + "TCPConnect"
	msg = append(msg, []byte(connectParams)...)
	msg = append(msg, 0x04) // EOT
	return msg
}

// RPCMessage builds the wire bytes for a no-argument RPC named name, in the post-
// handshake form (header 1130 + RPC chunk + empty param chunk). The trailing "54f"
// empty param chunk is REQUIRED: without a type-5 chunk XWB("PARAM") is never set and
// CALLP^XWBPRS's `D CAPI(.XWBP,XWB("PARAM"))` would UNDEF inside the broker.
func RPCMessage(name string) []byte {
	msg := []byte("[XWB]1130") // marker + header (VER=1,TYPE=1,LENV=3,RT=0)
	msg = append(msg, '2')     // chunk type 2 = RPC
	msg = append(msg, sread("0")...)
	msg = append(msg, sread(name)...)
	msg = append(msg, '5', '4', 'f') // empty param chunk: type 5, TY 4 (empty), CONT 'f'
	msg = append(msg, 0x04)          // EOT
	return msg
}

// Bye builds the "#BYE#" message that cleanly closes a broker session (the MAIN loop
// exits on it, XWBTCPM:182). Sending it lets the server tear down its job normally.
func Bye() []byte {
	msg := []byte("[XWB]1030") // marker + header
	msg = append(msg, '4')     // chunk type 4 = Command
	msg = append(msg, sread("#BYE#")...)
	msg = append(msg, 0x04) // EOT
	return msg
}
