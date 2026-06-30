package xwbwire

import (
	"bytes"
	"testing"
)

func TestRPCMessage(t *testing.T) {
	got := RPCMessage("XWB IM HERE")
	if !bytes.HasPrefix(got, []byte("[XWB]0030")) {
		t.Errorf("missing marker+header: %q", got)
	}
	if got[len(got)-1] != 0x04 {
		t.Errorf("missing EOT terminator: %q", got)
	}
	// chunk: '2' then SREAD("0") then SREAD(name)
	want := append([]byte("[XWB]00302"), byte(1), '0')
	want = append(want, byte(len("XWB IM HERE")))
	want = append(want, []byte("XWB IM HERE")...)
	want = append(want, 0x04)
	if !bytes.Equal(got, want) {
		t.Errorf("RPCMessage =\n %q\nwant\n %q", got, want)
	}
}

func TestRPCMessageLengthPrefixedName(t *testing.T) {
	got := RPCMessage("AB")
	// the name chunk SREAD("AB") = [2 'A' 'B'], immediately before the EOT
	if !bytes.Equal(got[len(got)-4:], []byte{2, 'A', 'B', 0x04}) {
		t.Errorf("name not length-prefixed correctly: %q", got)
	}
}
