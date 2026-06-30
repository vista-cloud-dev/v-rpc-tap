package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/vista-cloud-dev/v-rpc-tap/internal/tap"
)

// fakeEngine is a no-engine drainer: it returns canned sessions and records every
// CommitTrim so the at-least-once ack-gating can be asserted.
type fakeEngine struct {
	sessions []tap.Session
	drainErr error
	trimErr  error
	trims    []trim
	drains   int
}

type trim struct{ job, seq int }

func (f *fakeEngine) Drain(_ context.Context, _, _ int) ([]tap.Session, error) {
	f.drains++
	if f.drainErr != nil {
		return nil, f.drainErr
	}
	return f.sessions, nil
}

func (f *fakeEngine) CommitTrim(_ context.Context, job, seq int) error {
	if f.trimErr != nil {
		return f.trimErr
	}
	f.trims = append(f.trims, trim{job, seq})
	return nil
}

func goodCfg() Config {
	return Config{Bucket: "vsl-rpc-tap", Partition: "aws-us-gov", Region: "us-gov-west-1", Prefix: "rpc"}
}

// two jobs: 100 (seqmax 3) with a complete record, 200 (seqmax 5) names-only, plus a
// drop on job 100.
func twoJobSessions() []tap.Session {
	return []tap.Session{
		{Inc: "iA", Job: 100, Msgs: []tap.Record{
			{Inc: "iA", Job: 100, Seq: 1, SeqMax: 3, Drop: 2, Mode: 2, RPC: "ORWU DT", Sub: map[string]string{"R": "1"}},
		}},
		{Inc: "iB", Job: 200, Msgs: []tap.Record{
			{Inc: "iB", Job: 200, Seq: 5, SeqMax: 5, Mode: 1, RPC: "ORWPT LIST"},
		}},
	}
}

func TestRunOnce_ShipsThenTrimsAfterAck(t *testing.T) {
	eng := &fakeEngine{sessions: twoJobSessions()}
	sink := NewMemSink()
	p, err := New(eng, sink, goodCfg(), "ydb")
	if err != nil {
		t.Fatal(err)
	}
	rep, err := p.RunOnce(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	// exactly one object shipped, under the GovCloud key scheme
	if len(sink.Objects) != 1 {
		t.Fatalf("got %d objects, want 1", len(sink.Objects))
	}
	wantKey := "rpc/engine=ydb/window=0000000000.ldjson"
	if rep.Key != wantKey {
		t.Errorf("key = %q, want %q", rep.Key, wantKey)
	}
	if _, ok := sink.Objects[wantKey]; !ok {
		t.Errorf("object not stored under %q (have %v)", wantKey, keys(sink.Objects))
	}

	// LDJSON: one line per record (2 records)
	lines := strings.Split(strings.TrimRight(string(sink.Objects[wantKey]), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d LDJSON lines, want 2", len(lines))
	}
	var rec lineRec
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("LDJSON line not valid JSON: %v", err)
	}
	if rec.Job != 100 || rec.RPC != "ORWU DT" || rec.Class != "complete" {
		t.Errorf("line 0 = %+v, want job 100 ORWU DT complete", rec)
	}

	// committrim each distinct job to its drained SeqMax, in job order, AFTER the put
	want := []trim{{100, 3}, {200, 5}}
	if len(eng.trims) != 2 || eng.trims[0] != want[0] || eng.trims[1] != want[1] {
		t.Errorf("trims = %v, want %v", eng.trims, want)
	}
	if rep.Records != 2 || rep.Sessions != 2 || rep.Dropped != 2 {
		t.Errorf("report = %+v, want 2 records / 2 sessions / 2 dropped", rep)
	}
}

// At-least-once (D12): if the sink Put fails, NOTHING is trimmed — the records stay in
// the ring for the next drain.
func TestRunOnce_PutFailureDoesNotTrim(t *testing.T) {
	eng := &fakeEngine{sessions: twoJobSessions()}
	sink := &failSink{}
	p, _ := New(eng, sink, goodCfg(), "ydb")
	if _, err := p.RunOnce(context.Background(), 0, 0); err == nil {
		t.Fatal("RunOnce should return the sink error")
	}
	if len(eng.trims) != 0 {
		t.Errorf("trimmed %v after a failed put — must preserve for re-drain", eng.trims)
	}
}

// An empty drain ships nothing and trims nothing.
func TestRunOnce_EmptyDrainNoOp(t *testing.T) {
	eng := &fakeEngine{sessions: nil}
	sink := NewMemSink()
	p, _ := New(eng, sink, goodCfg(), "ydb")
	rep, err := p.RunOnce(context.Background(), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(sink.Objects) != 0 || len(eng.trims) != 0 || rep.Key != "" {
		t.Errorf("empty drain should be a no-op: objects=%d trims=%d key=%q", len(sink.Objects), len(eng.trims), rep.Key)
	}
}

// A bad (non-GovCloud) config is rejected at construction.
func TestNew_RejectsNonGovCloud(t *testing.T) {
	if _, err := New(&fakeEngine{}, NewMemSink(), Config{Bucket: "b", Partition: "aws", Region: "us-east-1"}, "ydb"); err == nil {
		t.Error("New should reject a commercial-partition config (U4)")
	}
}

// The window counter advances per shipped object, so successive drains get distinct keys.
func TestRunOnce_WindowAdvances(t *testing.T) {
	eng := &fakeEngine{sessions: twoJobSessions()}
	sink := NewMemSink()
	p, _ := New(eng, sink, goodCfg(), "ydb")
	r0, _ := p.RunOnce(context.Background(), 0, 0)
	r1, _ := p.RunOnce(context.Background(), 0, 0)
	if r0.Key == r1.Key {
		t.Errorf("successive windows share a key %q", r0.Key)
	}
	if r1.Window != 1 {
		t.Errorf("second window = %d, want 1", r1.Window)
	}
}

type failSink struct{}

func (failSink) Put(context.Context, string, []byte) error { return errors.New("egress down") }

func keys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
