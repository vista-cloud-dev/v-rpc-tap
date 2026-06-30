// Package host drives the VSL RPC TAP's engine-side host seam (VSLRTH) from Go:
// arm / disarm / status / drain / committrim, each one M command evaluated over the
// Execer seam, with drain output parsed + correlated through internal/tap. The
// production Execer wraps mdriver.Client — the only sanctioned transport (the m/v
// waterline, rule 3); the host never hand-rolls transport. Tests use a fake Execer,
// so the command-generation + response-parsing logic is verified with no engine.
package host

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/vista-cloud-dev/v-rpc-tap/internal/tap"
)

// Execer runs one M command on a live engine and returns its device output. It is
// the single seam the host reaches the engine through; the production implementation
// wraps mdriver.Client.
type Execer interface {
	Exec(ctx context.Context, command string) (string, error)
}

// Tap is the host-side controller for the VSL RPC TAP, driving VSLRTH over an Execer.
type Tap struct {
	eng Execer
}

// New returns a Tap driven by eng.
func New(eng Execer) *Tap { return &Tap{eng: eng} }

// Status is the parsed VSLRTH status line.
type Status struct {
	On      string // mode flag ("" = disarmed, "1"/"2" = armed)
	Epoch   string // arm $H marker
	Jobs    int    // job rings present
	Records int    // live records across all rings
}

// Arm turns capture on at mode (1 names-only / 2 full-payload) with a ttl-second host
// lease and a dur-second absolute cap (0 = indefinite).
func (t *Tap) Arm(ctx context.Context, mode, ttl, dur int) error {
	_, err := t.eng.Exec(ctx, fmt.Sprintf("do arm^VSLRTH(%d,%d,%d)", mode, ttl, dur))
	return err
}

// Disarm turns capture off; the reaper self-terminates on its next wake.
func (t *Tap) Disarm(ctx context.Context) error {
	_, err := t.eng.Exec(ctx, "do disarm^VSLRTH()")
	return err
}

// Status reports the armed state + job/record counts.
func (t *Tap) Status(ctx context.Context) (Status, error) {
	out, err := t.eng.Exec(ctx, "write $$status^VSLRTH()")
	if err != nil {
		return Status{}, err
	}
	return parseStatus(out)
}

// Drain pulls the live records for jobs in [lo,hi] (0,0 = all), sets the drained
// watermark but deletes nothing (D12), and returns them correlated into sessions.
func (t *Tap) Drain(ctx context.Context, lo, hi int) ([]tap.Session, error) {
	out, err := t.eng.Exec(ctx, fmt.Sprintf("do drain^VSLRTH(%d,%d)", lo, hi))
	if err != nil {
		return nil, err
	}
	recs, err := tap.ParseDrain(strings.NewReader(out))
	if err != nil {
		return nil, fmt.Errorf("parse drain: %w", err)
	}
	return tap.Correlate(recs), nil
}

// CommitTrim deletes the durably-stored prefix [head..seq] for job and advances head
// (call only after the drained records are safe in S3 — at-least-once, D12).
func (t *Tap) CommitTrim(ctx context.Context, job, seq int) error {
	_, err := t.eng.Exec(ctx, fmt.Sprintf("do committrim^VSLRTH(%d,%d)", job, seq))
	return err
}

// parseStatus reads the VSLRTH status line "on=<m>^epoch=<$H>^jobs=<n>^records=<n>".
func parseStatus(out string) (Status, error) {
	var st Status
	for _, field := range strings.Split(strings.TrimSpace(out), "^") {
		k, v, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		switch k {
		case "on":
			st.On = v
		case "epoch":
			st.Epoch = v
		case "jobs":
			n, err := strconv.Atoi(v)
			if err != nil {
				return Status{}, fmt.Errorf("status jobs %q: %w", v, err)
			}
			st.Jobs = n
		case "records":
			n, err := strconv.Atoi(v)
			if err != nil {
				return Status{}, fmt.Errorf("status records %q: %w", v, err)
			}
			st.Records = n
		}
	}
	return st, nil
}
