package rpctapcli

import (
	"bytes"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/vista-cloud-dev/clikit"
)

// fakeBroker is an in-process TCP server that accepts the TCPConnect handshake then
// replies to each RPC, so the load verb exercises the real handshake/fire client path
// with no engine.
func fakeBroker(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, rerr := c.Read(buf)
					if n > 0 {
						msg := buf[:n]
						switch {
						case bytes.Contains(msg, []byte("TCPConnect")):
							_, _ = c.Write([]byte("\x00\x00accept\x04"))
						case bytes.Contains(msg, []byte("#BYE#")):
							_, _ = c.Write([]byte("\x00\x00#BYE#\x04"))
							return
						default:
							_, _ = c.Write([]byte("\x00\x00DATA\x04"))
						}
					}
					if rerr != nil {
						return
					}
				}
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func TestLoadVerb_DrivesAndReports(t *testing.T) {
	c := &loadCmd{Addr: fakeBroker(t), Concurrency: 4, Total: 20, Timeout: 2 * time.Second}
	var buf bytes.Buffer
	if err := c.Run(jsonCtx(&buf)); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got struct {
		Sent       int     `json:"sent"`
		Failed     int     `json:"failed"`
		Throughput float64 `json:"throughputPerSec"`
	}
	decode(t, &buf, &got)
	if got.Sent != 20 || got.Failed != 0 {
		t.Errorf("result = %+v, want 20 sent / 0 failed", got)
	}
	if got.Throughput <= 0 {
		t.Errorf("throughput = %v, want > 0", got.Throughput)
	}
}

// A bad config (no total and no duration) surfaces as a clikit usage error, not a panic.
func TestLoadVerb_BadConfigIsUsageError(t *testing.T) {
	c := &loadCmd{Addr: "127.0.0.1:1", Concurrency: 1, Total: 0, Duration: 0}
	err := c.Run(jsonCtx(&bytes.Buffer{}))
	var ce *clikit.Error
	if !errors.As(err, &ce) || ce.Exit != clikit.ExitUsage {
		t.Errorf("want a clikit ExitUsage error, got %v", err)
	}
}
