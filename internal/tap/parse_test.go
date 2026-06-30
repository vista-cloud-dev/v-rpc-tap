package tap

import (
	"strings"
	"testing"
)

// drainFixture is a representative VSLRTH drain: job 100 has a complete mode-2 RPC
// (seq 1), an unpaired/denied one (seq 2), and an empty-name one (seq 3); job 200 is
// a names-only (mode 1) session.
func drainFixture() string {
	s := subSep
	lines := []string{
		"J\t100\t67751,40000-1\t1\t3",
		"V\t100\t1\t\t1^req^67751,40001^ORWU DT^2",
		"V\t100\t1\tP" + s + "1\tlit",
		"V\t100\t1\tR\t1",
		"V\t100\t1\tR" + s + "1\t3070101",
		"V\t100\t2\t\t1^req^67751,40002^XUS AV CODE^2",
		"V\t100\t3\t\t1^req^67751,40003^^2",
		"J\t200\t67751,40000-2\t5\t5",
		"V\t200\t5\t\t1^req^67751,40005^ORWPT LIST^1",
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestParseDrain(t *testing.T) {
	recs, err := ParseDrain(strings.NewReader(drainFixture()))
	if err != nil {
		t.Fatalf("ParseDrain: %v", err)
	}
	if len(recs) != 4 {
		t.Fatalf("got %d records, want 4", len(recs))
	}

	// records are sorted by (job, seq)
	r0 := recs[0]
	if r0.Job != 100 || r0.Seq != 1 || r0.Inc != "67751,40000-1" || r0.Head != 1 || r0.SeqMax != 3 {
		t.Errorf("r0 header wrong: %+v", r0)
	}
	if r0.Ver != 1 || r0.Kind != "req" || r0.Horolog != "67751,40001" || r0.RPC != "ORWU DT" || r0.Mode != 2 {
		t.Errorf("r0 base parse wrong: %+v", r0)
	}
	if !r0.HasResult() || r0.Class() != ClassComplete {
		t.Errorf("r0 should be complete: class=%v hasResult=%v", r0.Class(), r0.HasResult())
	}
	if r0.Sub["P"+subSep+"1"] != "lit" || r0.Sub["R"] != "1" || r0.Sub["R"+subSep+"1"] != "3070101" {
		t.Errorf("r0 sub-nodes wrong: %v", r0.Sub)
	}

	if recs[1].RPC != "XUS AV CODE" || recs[1].Class() != ClassUnpaired {
		t.Errorf("r1 should be unpaired XUS AV CODE: %+v", recs[1])
	}
	if recs[2].Named() || recs[2].Class() != ClassUnpaired {
		t.Errorf("r2 should be unnamed unpaired: %+v", recs[2])
	}
	if recs[3].Job != 200 || recs[3].Mode != 1 || recs[3].Class() != ClassNamesOnly {
		t.Errorf("r3 should be names-only job 200: %+v", recs[3])
	}
}

func TestParseDrainEmbeddedTabInValue(t *testing.T) {
	in := "V\t100\t1\t\t1^req^H^RPC^2\n" +
		"V\t100\t1\tP" + subSep + "1\ta\tb\tc\n"
	recs, err := ParseDrain(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseDrain: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if got := recs[0].Sub["P"+subSep+"1"]; got != "a\tb\tc" {
		t.Errorf("embedded tabs lost: %q", got)
	}
}

func TestParseDrainEmpty(t *testing.T) {
	recs, err := ParseDrain(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseDrain(empty): %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("got %d records from empty drain, want 0", len(recs))
	}
}

func TestParseDrainMalformedErrors(t *testing.T) {
	for _, in := range []string{
		"V\t100\tNOTANUM\t\tval\n", // seq not numeric
		"J\tNOTANUM\ti\t1\t3\n",    // job not numeric
		"V\tonly\ttwo\n",           // too few fields
	} {
		if _, err := ParseDrain(strings.NewReader(in)); err == nil {
			t.Errorf("ParseDrain(%q) = nil error, want a parse error", in)
		}
	}
}

func TestParseDrainEmbeddedNewlineContinuation(t *testing.T) {
	in := "V\t100\t1\t\t1^req^H^RPC^2\n" +
		"V\t100\t1\tR" + subSep + "1\tline-one\n" +
		"continued-line-two\n"
	recs, err := ParseDrain(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseDrain: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if got, want := recs[0].Sub["R"+subSep+"1"], "line-one\ncontinued-line-two"; got != want {
		t.Errorf("continuation not joined: got %q want %q", got, want)
	}
}
