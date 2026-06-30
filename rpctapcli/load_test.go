package rpctapcli

import (
	"bytes"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/vista-cloud-dev/clikit"
)

// fakeBroker is an in-process TCP server: it reads the [XWB] message and replies, so
// the load verb exercises the real client path with no engine.
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
				_, _ = c.Read(make([]byte, 256))
				_, _ = c.Write([]byte("\x00reject\x04"))
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
