package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/vista-cloud-dev/v-rpc-tap/internal/tap"
)

// drainer is the slice of the host controller (host.Tap) the pipeline drives. The
// interface exists so the flow is testable with a fake; *host.Tap satisfies it.
type drainer interface {
	Drain(ctx context.Context, lo, hi int) ([]tap.Session, error)
	CommitTrim(ctx context.Context, job, seq int) error
}

// Pipeline runs the drain → ship → committrim-after-ack flow against one engine.
type Pipeline struct {
	eng    drainer
	sink   Sink
	cfg    Config
	engine string
	window int
}

// New returns a Pipeline, rejecting a non-GovCloud config up front (U4).
func New(eng drainer, sink Sink, cfg Config, engine string) (*Pipeline, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Pipeline{eng: eng, sink: sink, cfg: cfg, engine: engine}, nil
}

// Report is the outcome of one drain window.
type Report struct {
	Key      string       // sink key written ("" if nothing shipped)
	Window   int          // window index for this drain
	Sessions int          // correlated sessions shipped
	Records  int          // records shipped
	Dropped  int          // pre-drain loss (per distinct job, R20)
	Bytes    int          // object size
	Trimmed  []TrimmedJob // jobs committed (only after a successful ship)
}

// TrimmedJob records a per-job trim committed after the ship was acknowledged.
type TrimmedJob struct {
	Job int `json:"job"`
	Seq int `json:"seq"`
}

// lineRec is one LDJSON line — one captured [XWB] message.
type lineRec struct {
	Inc    string            `json:"inc"`
	Job    int               `json:"job"`
	Seq    int               `json:"seq"`
	RPC    string            `json:"rpc"`
	Mode   int               `json:"mode"`
	Class  string            `json:"class"`
	Result bool              `json:"result"`
	Sub    map[string]string `json:"sub,omitempty"`
}

// RunOnce drains [lo,hi] (0,0 = all), ships one LDJSON object to the sink, and ONLY on
// a successful Put commits the engine-side trim for each drained job to its drained
// SeqMax (D12, at-least-once). A failed Put trims nothing — the records remain for the
// next drain. An empty drain ships and trims nothing.
func (p *Pipeline) RunOnce(ctx context.Context, lo, hi int) (Report, error) {
	sessions, err := p.eng.Drain(ctx, lo, hi)
	if err != nil {
		return Report{}, fmt.Errorf("drain: %w", err)
	}
	rep := Report{Window: p.window, Sessions: len(sessions)}
	if len(sessions) == 0 {
		return rep, nil // nothing to ship; window does not advance
	}

	body, sum := encodeLDJSON(sessions)
	key := p.key()
	if err := p.sink.Put(ctx, key, body); err != nil {
		// NOT acknowledged → trim nothing; the ring keeps the records for re-drain.
		return Report{}, fmt.Errorf("sink put %s: %w", key, err)
	}

	// Ack received → commit the trim per distinct job (deterministic job order).
	for _, js := range jobSeqMax(sessions) {
		if err := p.eng.CommitTrim(ctx, js.Job, js.Seq); err != nil {
			rep.Key, rep.Bytes, rep.Records, rep.Dropped = key, len(body), sum.Records, sum.Dropped
			return rep, fmt.Errorf("committrim job %d seq %d (object %s is durable; ring not trimmed past this): %w", js.Job, js.Seq, key, err)
		}
		rep.Trimmed = append(rep.Trimmed, js)
	}

	rep.Key, rep.Bytes, rep.Records, rep.Dropped = key, len(body), sum.Records, sum.Dropped
	p.window++
	return rep, nil
}

// key builds the GovCloud object key for the current window.
func (p *Pipeline) key() string {
	k := fmt.Sprintf("engine=%s/window=%010d.ldjson", p.engine, p.window)
	if p.cfg.Prefix != "" {
		return strings.TrimSuffix(p.cfg.Prefix, "/") + "/" + k
	}
	return k
}

// encodeLDJSON serializes the sessions as newline-delimited JSON (one line per record)
// and returns the per-window accounting alongside.
func encodeLDJSON(sessions []tap.Session) ([]byte, tap.Summary) {
	var b strings.Builder
	for _, s := range sessions {
		for _, m := range s.Msgs {
			line, _ := json.Marshal(lineRec{
				Inc: m.Inc, Job: m.Job, Seq: m.Seq, RPC: m.RPC, Mode: m.Mode,
				Class: m.Class().String(), Result: m.HasResult(), Sub: m.Sub,
			})
			b.Write(line)
			b.WriteByte('\n')
		}
	}
	return []byte(b.String()), tap.Summarize(sessions)
}

// jobSeqMax returns the drained SeqMax per distinct job, in ascending job order — the
// seq each job is committed (trimmed) to once the ship is acknowledged.
func jobSeqMax(sessions []tap.Session) []TrimmedJob {
	seqByJob := map[int]int{}
	for _, s := range sessions {
		for _, m := range s.Msgs {
			if m.SeqMax > seqByJob[m.Job] {
				seqByJob[m.Job] = m.SeqMax
			}
		}
	}
	out := make([]TrimmedJob, 0, len(seqByJob))
	for job, seq := range seqByJob {
		out = append(out, TrimmedJob{Job: job, Seq: seq})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Job < out[j].Job })
	return out
}
