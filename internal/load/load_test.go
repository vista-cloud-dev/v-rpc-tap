package load

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

// fakeBroker is an in-process TCP server standing in for the RPC broker: it reads one
// [XWB] message and replies with a short reject, so the harness exercises the real
// dial/write/read path with no live engine. delay simulates per-call cost.
func fakeBroker(t *testing.T, delay time.Duration) string {
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
				return // listener closed
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 256)
				_, _ = c.Read(buf)
				if delay > 0 {
					time.Sleep(delay)
				}
				_, _ = c.Write([]byte("\x00\x00reject\x04"))
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func TestRun_DrivesLoadAndInstruments(t *testing.T) {
	addr := fakeBroker(t, 0)
	cfg := Config{Addr: addr, Concurrency: 4, Total: 40, Timeout: 2 * time.Second, Mix: []string{"XWB IM HERE", "XUS INTRO MSG"}}
	rep, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Sent != 40 || rep.Failed != 0 {
		t.Errorf("sent=%d failed=%d, want 40/0", rep.Sent, rep.Failed)
	}
	if rep.Concurrency != 4 {
		t.Errorf("concurrency = %d, want 4", rep.Concurrency)
	}
	if rep.Throughput <= 0 {
		t.Errorf("throughput = %v, want > 0", rep.Throughput)
	}
	if rep.P50 <= 0 || rep.Max < rep.P50 {
		t.Errorf("latency percentiles incoherent: p50=%v max=%v", rep.P50, rep.Max)
	}
}

func TestRun_CountsFailuresAgainstDeadEndpoint(t *testing.T) {
	// nothing listening on this port → every dial fails, none silently dropped
	cfg := Config{Addr: "127.0.0.1:1", Concurrency: 2, Total: 6, Timeout: 200 * time.Millisecond, Mix: []string{"X"}}
	rep, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Failed != 6 || rep.Sent != 0 {
		t.Errorf("failed=%d sent=%d, want 6/0", rep.Failed, rep.Sent)
	}
}

func TestRun_DurationMode(t *testing.T) {
	addr := fakeBroker(t, 0)
	cfg := Config{Addr: addr, Concurrency: 2, Duration: 150 * time.Millisecond, Timeout: time.Second, Mix: []string{"X"}}
	rep, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Sent == 0 {
		t.Error("duration-mode run fired no RPCs")
	}
	if rep.Elapsed < 100*time.Millisecond {
		t.Errorf("elapsed = %v, want ~the configured duration", rep.Elapsed)
	}
}

func TestRun_RejectsBadConfig(t *testing.T) {
	for _, cfg := range []Config{
		{Addr: "", Concurrency: 1, Total: 1, Mix: []string{"X"}},    // no addr
		{Addr: "h:1", Concurrency: 0, Total: 1, Mix: []string{"X"}}, // no workers
		{Addr: "h:1", Concurrency: 1, Mix: []string{"X"}},           // neither Total nor Duration
		{Addr: "h:1", Concurrency: 1, Total: 1},                     // empty mix
	} {
		if _, err := Run(context.Background(), cfg); err == nil {
			t.Errorf("Run(%+v) = nil error, want a config error", cfg)
		}
	}
}

func TestSummarize_Percentiles(t *testing.T) {
	// 10 samples 10ms..100ms, all ok; elapsed 1s → throughput 10/s
	var s []sample
	for i := 1; i <= 10; i++ {
		s = append(s, sample{lat: time.Duration(i) * 10 * time.Millisecond})
	}
	rep := summarize(s, time.Second, 5)
	if rep.Sent != 10 || rep.Failed != 0 {
		t.Fatalf("sent=%d failed=%d, want 10/0", rep.Sent, rep.Failed)
	}
	if rep.Throughput != 10 {
		t.Errorf("throughput = %v, want 10", rep.Throughput)
	}
	if rep.Min != 10*time.Millisecond || rep.Max != 100*time.Millisecond {
		t.Errorf("min=%v max=%v, want 10ms/100ms", rep.Min, rep.Max)
	}
	// nearest-rank: p50 of 10 sorted → index ceil(0.50*10)=5 → 50ms; p95 → ceil(9.5)=10 → 100ms
	if rep.P50 != 50*time.Millisecond {
		t.Errorf("p50 = %v, want 50ms", rep.P50)
	}
	if rep.P95 != 100*time.Millisecond {
		t.Errorf("p95 = %v, want 100ms", rep.P95)
	}
}

func TestSummarize_CountsFailures(t *testing.T) {
	s := []sample{{lat: 5 * time.Millisecond}, {err: io.EOF}, {err: io.EOF}}
	rep := summarize(s, time.Second, 1)
	if rep.Sent != 1 || rep.Failed != 2 {
		t.Errorf("sent=%d failed=%d, want 1/2", rep.Sent, rep.Failed)
	}
}

func TestDelta_ComputesArmedOverhead(t *testing.T) {
	control := Report{P50: 10 * time.Millisecond, P95: 20 * time.Millisecond, Mean: 12 * time.Millisecond, Throughput: 100}
	armed := Report{P50: 13 * time.Millisecond, P95: 26 * time.Millisecond, Mean: 15 * time.Millisecond, Throughput: 90}
	d := Delta(control, armed)
	if d.P50 != 3*time.Millisecond || d.P95 != 6*time.Millisecond || d.Mean != 3*time.Millisecond {
		t.Errorf("delta = %+v, want p50 3ms / p95 6ms / mean 3ms", d)
	}
	if d.ThroughputDropPct < 9.9 || d.ThroughputDropPct > 10.1 {
		t.Errorf("throughput drop = %.2f%%, want ~10%%", d.ThroughputDropPct)
	}
}
