package rpctapcli

import (
	"context"
	"fmt"
	"strconv"

	"github.com/vista-cloud-dev/clikit"
	"github.com/vista-cloud-dev/v-rpc-tap/internal/host"
	"github.com/vista-cloud-dev/v-rpc-tap/internal/tap"
)

// The `v rpc-tap` control verbs drive the engine-side host seam (VSLRTH) through the
// host.Tap controller over the mdriver-backed Execer. Each verb resolves the Execer
// (engineConn.execer), builds the controller, then delegates to a thin `exec` method
// that holds the testable logic — exercised with a fake Execer, no engine. The live
// smoke (these verbs against VSLRTH actually installed on vehu/foia) is DEFERRED: it
// is gated on the v-pkg install path (the ZVPKGRD staging snag), which is the owner's
// parallel reliability work — do not couple to it here.

// --- arm --------------------------------------------------------------------

type armCmd struct {
	engineConn
	Mode int `help:"Capture mode: 1=names-only, 2=full-payload (PHI)." default:"2" enum:"1,2"`
	TTL  int `help:"Host lease in seconds; the reaper disarms if the host stops renewing." default:"90"`
	Dur  int `help:"Absolute arm cap in seconds (0 = indefinite)." default:"0"`
}

func (c *armCmd) Run(cc *clikit.Context) error { return run(cc, c.engineConn, c.exec) }

func (c *armCmd) exec(cc *clikit.Context, t *host.Tap) error {
	if err := t.Arm(context.Background(), c.Mode, c.TTL, c.Dur); err != nil {
		return clikit.Fail(clikit.ExitRuntime, "ARM", err.Error(), "")
	}
	return cc.Result(
		struct {
			Engine string `json:"engine"`
			Mode   int    `json:"mode"`
			TTL    int    `json:"ttl"`
			Dur    int    `json:"dur"`
		}{c.Engine, c.Mode, c.TTL, c.Dur},
		func() {
			fmt.Fprintf(cc.Stdout, "VSL RPC TAP armed mode %d on %s (lease %ds, cap %ds)\n",
				c.Mode, c.Engine, c.TTL, c.Dur)
		},
	)
}

// --- disarm -----------------------------------------------------------------

type disarmCmd struct {
	engineConn
}

func (c *disarmCmd) Run(cc *clikit.Context) error { return run(cc, c.engineConn, c.exec) }

func (c *disarmCmd) exec(cc *clikit.Context, t *host.Tap) error {
	if err := t.Disarm(context.Background()); err != nil {
		return clikit.Fail(clikit.ExitRuntime, "DISARM", err.Error(), "")
	}
	return cc.Result(
		struct {
			Engine string `json:"engine"`
		}{c.Engine},
		func() { fmt.Fprintf(cc.Stdout, "VSL RPC TAP disarmed on %s\n", c.Engine) },
	)
}

// --- status -----------------------------------------------------------------

type statusCmd struct {
	engineConn
}

func (c *statusCmd) Run(cc *clikit.Context) error { return run(cc, c.engineConn, c.exec) }

func (c *statusCmd) exec(cc *clikit.Context, t *host.Tap) error {
	st, err := t.Status(context.Background())
	if err != nil {
		return clikit.Fail(clikit.ExitRuntime, "ENGINE", err.Error(),
			"is the engine up and reachable over the driver?")
	}
	mode, _ := strconv.Atoi(st.On) // "" → 0 (disarmed); "1"/"2" → the armed mode
	data := struct {
		Engine  string `json:"engine"`
		Armed   bool   `json:"armed"`
		Mode    int    `json:"mode"`
		Epoch   string `json:"epoch"`
		Jobs    int    `json:"jobs"`
		Records int    `json:"records"`
	}{c.Engine, st.On != "", mode, st.Epoch, st.Jobs, st.Records}
	return cc.Result(data, func() {
		armed := "off"
		if data.Armed {
			armed = fmt.Sprintf("ON mode %d", data.Mode)
		}
		fmt.Fprintf(cc.Stdout, "engine %s: VSL RPC TAP %s; %d job ring(s), %d live record(s)\n",
			data.Engine, armed, data.Jobs, data.Records)
	})
}

// --- drain ------------------------------------------------------------------

type drainCmd struct {
	engineConn
	Lo int `help:"Low job number of the drain range (0 = from the first job)." default:"0"`
	Hi int `help:"High job number of the drain range (0 = through the last job)." default:"0"`
}

func (c *drainCmd) Run(cc *clikit.Context) error { return run(cc, c.engineConn, c.exec) }

func (c *drainCmd) exec(cc *clikit.Context, t *host.Tap) error {
	sessions, err := t.Drain(context.Background(), c.Lo, c.Hi)
	if err != nil {
		return clikit.Fail(clikit.ExitRuntime, "DRAIN", err.Error(), "")
	}
	sum := tap.Summarize(sessions)
	data := struct {
		Engine    string `json:"engine"`
		Sessions  int    `json:"sessions"`
		Records   int    `json:"records"`
		Complete  int    `json:"complete"`
		Unpaired  int    `json:"unpaired"`
		NamesOnly int    `json:"namesOnly"`
		EmptyName int    `json:"emptyName"`
		Dropped   int    `json:"dropped"`
	}{c.Engine, sum.Sessions, sum.Records, sum.Complete, sum.Unpaired, sum.NamesOnly, sum.EmptyName, sum.Dropped}
	return cc.Result(data, func() {
		fmt.Fprintf(cc.Stdout,
			"drained %s: %d session(s), %d record(s) — %d complete, %d unpaired, %d names-only, %d empty-name; %d dropped (pre-drain loss)\n",
			data.Engine, data.Sessions, data.Records, data.Complete, data.Unpaired, data.NamesOnly, data.EmptyName, data.Dropped)
	})
}

// --- committrim -------------------------------------------------------------

type commitTrimCmd struct {
	engineConn
	Job int `help:"The broker $J whose durable ring prefix to trim." required:""`
	Seq int `help:"Trim the durably-stored prefix [head..seq] (call only after the drain is safe in S3)." required:""`
}

func (c *commitTrimCmd) Run(cc *clikit.Context) error { return run(cc, c.engineConn, c.exec) }

func (c *commitTrimCmd) exec(cc *clikit.Context, t *host.Tap) error {
	if err := t.CommitTrim(context.Background(), c.Job, c.Seq); err != nil {
		return clikit.Fail(clikit.ExitRuntime, "COMMITTRIM", err.Error(), "")
	}
	return cc.Result(
		struct {
			Engine string `json:"engine"`
			Job    int    `json:"job"`
			Seq    int    `json:"seq"`
		}{c.Engine, c.Job, c.Seq},
		func() { fmt.Fprintf(cc.Stdout, "trimmed job %d through seq %d on %s\n", c.Job, c.Seq, c.Engine) },
	)
}

// run is the shared verb prologue: resolve the Execer over the driver seam, build the
// host controller, and hand it to the verb's exec. Keeping it separate lets exec be
// table-tested with a fake Execer (no engine).
func run(cc *clikit.Context, ec engineConn, exec func(*clikit.Context, *host.Tap) error) error {
	ex, ferr := ec.execer()
	if ferr != nil {
		return ferr
	}
	return exec(cc, host.New(ex))
}
