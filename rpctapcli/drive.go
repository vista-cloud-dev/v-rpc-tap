package rpctapcli

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/vista-cloud-dev/clikit"
	"github.com/vista-cloud-dev/v-rpc-tap/internal/broker"
)

// driveCmd opens ONE broker session (full TCPConnect handshake) and fires a sequence of
// RPCs through MAIN->CALLP^XWBPRS — the splice point. Unlike `load` (throughput at
// concurrency), `drive` is the controllable single-session driver for the live L1-L3
// safety proofs: it reports each RPC's exact response bytes so a control run (unspliced
// / disarmed) and an armed run can be diffed to prove the tap adds ZERO wire bytes
// (CF4) and never makes the broker send an error packet. It is an RPC *client* (broker
// --addr), not the engine seam.
type driveCmd struct {
	Addr    string        `help:"Broker host:port to drive; vehu=127.0.0.1:9430 ($VRPCTAP_ADDR)." default:"127.0.0.1:9430" placeholder:"HOST:PORT" env:"VRPCTAP_ADDR"`
	RPC     []string      `help:"RPC name to fire (repeatable, in order)." placeholder:"NAME"`
	Timeout time.Duration `help:"Per-connection dial/read timeout." default:"5s"`
}

// driveResult is one fired RPC's outcome: the response length + hex (for byte-exact
// control-vs-armed diffing).
type driveResult struct {
	RPC     string `json:"rpc"`
	RespLen int    `json:"respLen"`
	RespHex string `json:"respHex"`
	Err     string `json:"err,omitempty"`
}

type driveReport struct {
	Addr    string        `json:"addr"`
	Fired   int           `json:"fired"`
	Results []driveResult `json:"results"`
}

func (c *driveCmd) Run(cc *clikit.Context) error {
	if len(c.RPC) == 0 {
		return clikit.Fail(clikit.ExitUsage, "USAGE", "no RPCs given", "pass at least one --rpc NAME")
	}
	s, err := broker.Dial(c.Addr, c.Timeout)
	if err != nil {
		return clikit.Fail(clikit.ExitRuntime, "BROKER", err.Error(),
			"is the broker listening on --addr? (vehu=127.0.0.1:9430, foia=127.0.0.1:19430)")
	}
	defer func() { _ = s.Close() }()

	rep := driveReport{Addr: c.Addr}
	for _, name := range c.RPC {
		resp, ferr := s.Fire(name)
		res := driveResult{RPC: name, RespLen: len(resp), RespHex: hex.EncodeToString(resp)}
		if ferr != nil {
			res.Err = ferr.Error()
		}
		rep.Results = append(rep.Results, res)
		rep.Fired++
	}
	return cc.Result(rep, func() {
		fmt.Fprintf(cc.Stdout, "drive %s: %d RPC(s)\n", rep.Addr, rep.Fired)
		for _, r := range rep.Results {
			status := fmt.Sprintf("%d bytes", r.RespLen)
			if r.Err != "" {
				status = "ERR: " + r.Err
			}
			fmt.Fprintf(cc.Stdout, "  %-28s %s\n", r.RPC, status)
		}
	})
}
