package rpctapcli

import (
	"context"
	"fmt"
	"time"

	"github.com/vista-cloud-dev/clikit"
	"github.com/vista-cloud-dev/v-rpc-tap/internal/load"
)

// defaultMix is a small set of harmless no-arg RPCs the broker logs by name then
// rejects (no signed-on session) — enough to drive the dispatch path under load.
var defaultMix = []string{"XWB IM HERE", "XUS INTRO MSG", "XWB GET VARIABLE VALUE"}

// loadCmd is the L8 load-harness rig: it drives concurrent [XWB] sessions at a broker
// TCP port and reports throughput + latency percentiles. It connects as an RPC client
// (like CPRS), NOT through the engine seam, so it takes a broker --addr, not the engine
// flags. Run it against an UNSPLICED broker for the control baseline; re-run with the
// tap armed (once the splice is installed) and diff to read off the latency tax.
type loadCmd struct {
	Addr        string        `help:"Broker host:port to drive; vehu=127.0.0.1:9430 ($VRPCTAP_ADDR)." default:"127.0.0.1:9430" placeholder:"HOST:PORT" env:"VRPCTAP_ADDR"`
	RPC         []string      `help:"RPC name in the workload mix; repeatable. Default: a small no-arg set." placeholder:"NAME"`
	Concurrency int           `help:"Concurrent broker sessions." default:"50"`
	Total       int           `help:"Total RPCs to fire across all sessions." default:"500"`
	Duration    time.Duration `help:"Run for this long instead of a fixed --total (e.g. 30s); overrides --total." default:"0"`
	Timeout     time.Duration `help:"Per-connection dial/read timeout." default:"3s"`
}

func (c *loadCmd) Run(cc *clikit.Context) error {
	mix := c.RPC
	if len(mix) == 0 {
		mix = defaultMix
	}
	cfg := load.Config{
		Addr:        c.Addr,
		Concurrency: c.Concurrency,
		Total:       c.Total,
		Duration:    c.Duration,
		Timeout:     c.Timeout,
		Mix:         mix,
	}
	if c.Duration > 0 { // --duration overrides the default --total
		cfg.Total = 0
	}
	rep, err := load.Run(context.Background(), cfg)
	if err != nil {
		return clikit.Fail(clikit.ExitUsage, "USAGE", err.Error(), "set --total or --duration, and a broker --addr")
	}
	return cc.Result(rep, func() {
		fmt.Fprintf(cc.Stdout,
			"load %s: %d sent / %d failed in %s (%.0f rpc/s, %d sessions)\n",
			rep.Addr, rep.Sent, rep.Failed, rep.Elapsed.Round(time.Millisecond), rep.Throughput, rep.Concurrency)
		fmt.Fprintf(cc.Stdout, "  latency  min %s  p50 %s  p95 %s  p99 %s  max %s\n",
			rep.Min.Round(time.Microsecond), rep.P50.Round(time.Microsecond), rep.P95.Round(time.Microsecond),
			rep.P99.Round(time.Microsecond), rep.Max.Round(time.Microsecond))
	})
}
