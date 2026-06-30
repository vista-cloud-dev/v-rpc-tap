package broker

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"
)

// fakeBroker is an in-process TCP server that speaks just enough of the [XWB] response
// framing (length-prefixed sec/err packets + data + 0x04 EOT) to drive the Session
// through a full handshake/fire/bye cycle. reply decides the response per request.
func fakeBroker(t *testing.T, reply func(msg []byte) []byte) (addr string, seen func() [][]byte) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	var mu sync.Mutex
	var msgs [][]byte
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			msg, err := readEOT(conn, 2*time.Second)
			if len(msg) > 0 {
				mu.Lock()
				msgs = append(msgs, append([]byte(nil), msg...))
				mu.Unlock()
			}
			if err != nil {
				return
			}
			if _, err := conn.Write(reply(msg)); err != nil {
				return
			}
			if bytes.Contains(msg, []byte("#BYE#")) {
				return
			}
		}
	}()

	return ln.Addr().String(), func() [][]byte {
		mu.Lock()
		defer mu.Unlock()
		out := make([][]byte, len(msgs))
		copy(out, msgs)
		return out
	}
}

func acceptingReply(msg []byte) []byte {
	switch {
	case bytes.Contains(msg, []byte("TCPConnect")):
		return []byte("\x00\x00accept\x04")
	case bytes.Contains(msg, []byte("#BYE#")):
		return []byte("\x00\x00#BYE#\x04")
	default:
		return []byte("\x00\x00OKDATA\x04")
	}
}

func TestSessionHandshakeFireBye(t *testing.T) {
	addr, seen := fakeBroker(t, acceptingReply)

	s, err := Dial(addr, 2*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	resp, err := s.Fire("ORWU DT")
	if err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if !bytes.Contains(resp, []byte("OKDATA")) {
		t.Errorf("response = %q, want it to contain OKDATA", resp)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	msgs := seen()
	if len(msgs) != 3 {
		t.Fatalf("server saw %d messages, want 3 (connect, rpc, bye)", len(msgs))
	}
	if !bytes.Contains(msgs[0], []byte("TCPConnect")) {
		t.Errorf("first message not TCPConnect: %q", msgs[0])
	}
	if !bytes.Contains(msgs[1], []byte("ORWU DT")) {
		t.Errorf("second message not the RPC: %q", msgs[1])
	}
	if !bytes.Contains(msgs[2], []byte("#BYE#")) {
		t.Errorf("third message not #BYE#: %q", msgs[2])
	}
}

func TestDialRejectedHandshake(t *testing.T) {
	addr, _ := fakeBroker(t, func(_ []byte) []byte {
		return []byte("\x00\x00reject\x04") // broker rejects (non-TCPConnect first msg)
	})
	if _, err := Dial(addr, 2*time.Second); err == nil {
		t.Fatal("expected an error when the broker rejects the handshake")
	}
}

func TestDialUnreachable(t *testing.T) {
	// 127.0.0.1:1 is reserved/closed — dial should fail fast, not hang.
	if _, err := Dial("127.0.0.1:1", 500*time.Millisecond); err == nil {
		t.Fatal("expected a dial error to a closed port")
	}
}
