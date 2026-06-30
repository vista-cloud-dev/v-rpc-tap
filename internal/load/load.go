// Package load is the L8 synthetic-[XWB] load-harness rig (R15): it drives N concurrent
// broker sessions firing a workload mix of RPCs over the [XWB] wire protocol and
// instruments per-call latency + throughput. It acts as an RPC *client* of the broker
// TCP socket (the CPRS-equivalent, like v-rpc-debug's ping) — it does NOT reach the M
// engine, so it takes a broker address, not the m-driver seam.
//
// Each worker holds ONE persistent broker.Session: it completes the TCPConnect handshake
// once, then fires its share of RPCs through that session (each Fire reaches MAIN->
// CALLP^XWBPRS, the splice point), then #BYE#s on close. This both models a real CPRS
// client (one session, many RPCs) and is what makes the ARMED run actually exercise the
// tap — a handshake-less per-RPC dial is rejected at NEW^XWBTCPM before CALLP (see
// internal/broker). Run the rig against an unspliced broker for the CONTROL baseline and
// again with the splice installed + armed for the ARMED run; Delta() reads off the tax.
package load

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vista-cloud-dev/v-rpc-tap/internal/broker"
)

// Config parameterizes a load run. Provide either Total (a fixed number of RPCs across
// all workers) or Duration (run until the deadline); Total wins if both are set.
type Config struct {
	Addr        string        // broker host:port (e.g. vehu 127.0.0.1:9430)
	Concurrency int           // number of concurrent sessions (workers)
	Total       int           // total RPCs to fire across all workers (0 = use Duration)
	Duration    time.Duration // run for this long instead of a fixed Total
	Timeout     time.Duration // per-connection dial/read timeout
	Mix         []string      // RPC names cycled as the workload (round-robin per worker)
}

func (c Config) validate() error {
	switch {
	case c.Addr == "":
		return fmt.Errorf("load config: addr is required")
	case c.Concurrency < 1:
		return fmt.Errorf("load config: concurrency must be >= 1")
	case c.Total <= 0 && c.Duration <= 0:
		return fmt.Errorf("load config: set Total or Duration")
	case len(c.Mix) == 0:
		return fmt.Errorf("load config: mix must name at least one RPC")
	}
	return nil
}

// sample is one fired RPC's outcome.
type sample struct {
	lat time.Duration
	err error
}

// Report is the per-run instrumentation.
type Report struct {
	Addr        string        `json:"addr"`
	Concurrency int           `json:"concurrency"`
	Sent        int           `json:"sent"`
	Failed      int           `json:"failed"`
	Elapsed     time.Duration `json:"elapsedNs"`
	Throughput  float64       `json:"throughputPerSec"` // successful RPCs / sec
	Min         time.Duration `json:"minNs"`
	Mean        time.Duration `json:"meanNs"`
	P50         time.Duration `json:"p50Ns"`
	P95         time.Duration `json:"p95Ns"`
	P99         time.Duration `json:"p99Ns"`
	Max         time.Duration `json:"maxNs"`
}

// Run drives the load and returns the instrumented report. Each worker establishes one
// persistent broker.Session (handshake) and fires its claimed RPCs through it; a worker
// that can't establish (or whose session breaks) still claims and FAILS its budget units
// so nothing is silently dropped.
func Run(ctx context.Context, cfg Config) (Report, error) {
	if err := cfg.validate(); err != nil {
		return Report{}, err
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if cfg.Total <= 0 { // duration mode: stop workers at the deadline
		runCtx, cancel = context.WithTimeout(ctx, cfg.Duration)
		defer cancel()
	}

	remaining := int64(cfg.Total) // total mode: shared work budget
	samples := make([][]sample, cfg.Concurrency)
	var wg sync.WaitGroup
	start := time.Now()
	for w := 0; w < cfg.Concurrency; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			// One session per worker: handshake once, fire many (real CPRS shape). A
			// dead deadErr makes every subsequently-claimed unit a failure.
			sess, deadErr := broker.Dial(cfg.Addr, cfg.Timeout)
			if sess != nil {
				defer func() { _ = sess.Close() }()
			}
			for i := 0; ; i++ {
				if cfg.Total > 0 {
					if atomic.AddInt64(&remaining, -1) < 0 {
						return // budget exhausted
					}
				} else if runCtx.Err() != nil {
					return // duration elapsed / canceled
				}
				name := cfg.Mix[i%len(cfg.Mix)]
				if deadErr != nil {
					samples[w] = append(samples[w], sample{err: deadErr}) // no session — fail the unit
					continue
				}
				t0 := time.Now()
				_, ferr := sess.Fire(name)
				samples[w] = append(samples[w], sample{lat: time.Since(t0), err: ferr})
				if ferr != nil {
					deadErr = ferr // session broken: remaining claimed units count as failed
				}
			}
		}(w)
	}
	wg.Wait()
	elapsed := time.Since(start)

	all := make([]sample, 0, cfg.Total)
	for _, s := range samples {
		all = append(all, s...)
	}
	rep := summarize(all, elapsed, cfg.Concurrency)
	rep.Addr = cfg.Addr
	return rep, nil
}

// summarize reduces samples into a Report: throughput over elapsed, and latency
// percentiles (nearest-rank) over the successful calls only.
func summarize(samples []sample, elapsed time.Duration, concurrency int) Report {
	rep := Report{Concurrency: concurrency, Elapsed: elapsed}
	lats := make([]time.Duration, 0, len(samples))
	var total time.Duration
	for _, s := range samples {
		if s.err != nil {
			rep.Failed++
			continue
		}
		rep.Sent++
		lats = append(lats, s.lat)
		total += s.lat
	}
	if elapsed > 0 {
		rep.Throughput = float64(rep.Sent) / elapsed.Seconds()
	}
	if len(lats) == 0 {
		return rep
	}
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	rep.Min = lats[0]
	rep.Max = lats[len(lats)-1]
	rep.Mean = total / time.Duration(len(lats))
	rep.P50 = percentile(lats, 0.50)
	rep.P95 = percentile(lats, 0.95)
	rep.P99 = percentile(lats, 0.99)
	return rep
}

// percentile returns the nearest-rank pth percentile of a sorted slice.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(p * float64(len(sorted))))
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

// DeltaReport is the control-vs-armed latency overhead + throughput cost (R15/R16).
type DeltaReport struct {
	P50               time.Duration `json:"p50Ns"`
	P95               time.Duration `json:"p95Ns"`
	Mean              time.Duration `json:"meanNs"`
	ThroughputDropPct float64       `json:"throughputDropPct"`
}

// Delta computes the armed run's overhead over the control run.
func Delta(control, armed Report) DeltaReport {
	d := DeltaReport{
		P50:  armed.P50 - control.P50,
		P95:  armed.P95 - control.P95,
		Mean: armed.Mean - control.Mean,
	}
	if control.Throughput > 0 {
		d.ThroughputDropPct = (control.Throughput - armed.Throughput) / control.Throughput * 100
	}
	return d
}
