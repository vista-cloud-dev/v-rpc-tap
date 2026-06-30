package xwbwire

import (
	"bytes"
	"testing"
)

// The wire bytes are grounded in XWBTCPMT.m (the VA-shipped "routine to test a
// connection"), which writes verbatim, working [XWB] messages. Reproducing those
// byte sequences is what lets the client complete the TCPConnect handshake and reach
// MAIN->CALLP^XWBPRS (where the tap is spliced) — the bare RPC chunk alone is rejected
// at the NEW handshake (XWBTCPM:137) and never reaches CALLP.

func TestConnectMatchesXWBTCPMT(t *testing.T) {
	// XWBTCPMT:31  W "[XWB]10304"_$C(10)_"TCPConnect50010127.0.0.1f00010f0024ISF-FORTW.vha.domain.extf"_$C(4)
	want := []byte("[XWB]10304")
	want = append(want, 0x0a)
	want = append(want, []byte("TCPConnect50010127.0.0.1f00010f0024ISF-FORTW.vha.domain.extf")...)
	want = append(want, 0x04)
	if got := Connect(); !bytes.Equal(got, want) {
		t.Errorf("Connect() =\n %q\nwant\n %q", got, want)
	}
}

func TestRPCMessageMatchesXWBTCPMTShape(t *testing.T) {
	// XWBTCPMT:34  W "[XWB]11302"_$C(1)_"0"_$C(16)_"XUS SIGNON SETUP54f"_$C(4)
	// header 1130 (VER=1,TYPE=1,LENV=3,RT=0) + '2' RPC chunk + SREAD("0") version
	// + SREAD(name) + "54f" empty param chunk (REQUIRED: sets XWB("PARAM")="") + EOT.
	want := []byte("[XWB]11302")
	want = append(want, 0x01, '0')                     // SREAD("0") version
	want = append(want, byte(len("XUS SIGNON SETUP"))) // SREAD len
	want = append(want, []byte("XUS SIGNON SETUP")...) // name
	want = append(want, '5', '4', 'f')                 // empty param chunk
	want = append(want, 0x04)                          // EOT
	if got := RPCMessage("XUS SIGNON SETUP"); !bytes.Equal(got, want) {
		t.Errorf("RPCMessage =\n %q\nwant\n %q", got, want)
	}
}

func TestRPCMessageNameLengthPrefixed(t *testing.T) {
	got := RPCMessage("AB")
	// the name SREAD("AB") = [2 'A' 'B'] sits right before the "54f" param chunk + EOT
	if !bytes.Equal(got[len(got)-7:], []byte{2, 'A', 'B', '5', '4', 'f', 0x04}) {
		t.Errorf("name/param tail wrong: %q", got)
	}
	if !bytes.HasPrefix(got, []byte("[XWB]11302")) {
		t.Errorf("missing marker+header+chunk: %q", got)
	}
}

func TestByeMatchesXWBTCPMT(t *testing.T) {
	// XWBTCPMT:37  W "[XWB]10304"_$C(5)_"#BYE#"_$C(4)
	want := []byte("[XWB]10304")
	want = append(want, 0x05)
	want = append(want, []byte("#BYE#")...)
	want = append(want, 0x04)
	if got := Bye(); !bytes.Equal(got, want) {
		t.Errorf("Bye() =\n %q\nwant\n %q", got, want)
	}
}
