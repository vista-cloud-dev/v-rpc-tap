package rpctapcli

import (
	"bytes"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/vista-cloud-dev/clikit"
)

// acceptingBroker is an in-process TCP server that accepts the TCPConnect handshake and
// replies to each RPC with a canned EOT-terminated data packet, so the drive verb
// exercises the full handshake/fire/bye client path with no engine.
func acceptingBroker(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 1024)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				msg := buf[:n]
				switch {
				case bytes.Contains(msg, []byte("TCPConnect")):
					_, _ = conn.Write([]byte("\x00\x00accept\x04"))
				case bytes.Contains(msg, []byte("#BYE#")):
					_, _ = conn.Write([]byte("\x00\x00#BYE#\x04"))
					return
				default:
					_, _ = conn.Write([]byte("\x00\x00DATA\x04"))
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return ln.Addr().String()
}

func TestDriveVerb_HandshakeAndFire(t *testing.T) {
	c := &driveCmd{Addr: acceptingBroker(t), RPC: []string{"XUS INTRO MSG", "ORWU DT"}, Timeout: 2 * time.Second}
	var buf bytes.Buffer
	if err := c.Run(jsonCtx(&buf)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got driveReport
	decode(t, &buf, &got)
	if got.Fired != 2 {
		t.Fatalf("fired = %d, want 2", got.Fired)
	}
	for _, r := range got.Results {
		if r.Err != "" {
			t.Errorf("%s errored: %s", r.RPC, r.Err)
		}
		if r.RespLen == 0 || r.RespHex == "" {
			t.Errorf("%s: empty response (%d bytes)", r.RPC, r.RespLen)
		}
	}
}

func TestDriveVerb_NoRPCsIsUsageError(t *testing.T) {
	c := &driveCmd{Addr: "127.0.0.1:1", Timeout: time.Second}
	err := c.Run(jsonCtx(&bytes.Buffer{}))
	var ce *clikit.Error
	if !errors.As(err, &ce) || ce.Exit != clikit.ExitUsage {
		t.Errorf("want clikit ExitUsage, got %v", err)
	}
}

func TestDriveVerb_UnreachableBrokerIsRuntimeError(t *testing.T) {
	c := &driveCmd{Addr: "127.0.0.1:1", RPC: []string{"X"}, Timeout: 500 * time.Millisecond}
	err := c.Run(jsonCtx(&bytes.Buffer{}))
	var ce *clikit.Error
	if !errors.As(err, &ce) || ce.Exit != clikit.ExitRuntime {
		t.Errorf("want clikit ExitRuntime, got %v", err)
	}
}
