package tap

import (
	"fmt"
	"strings"
	"testing"
)

// jhdr / vnode build the length-prefixed drain wire format (VSLRTH.m's $$jhdr/$$vhdr +
// the value line), so fixtures stay readable and the byte length is never hand-counted.
func jhdr(job int, inc string, head, seqmax, drop int) string {
	return fmt.Sprintf("J\t%d\t%s\t%d\t%d\t%d\n", job, inc, head, seqmax, drop)
}

func vnode(job, seq int, sub, val string) string {
	return fmt.Sprintf("V\t%d\t%d\t%s\t%d\n%s\n", job, seq, sub, len(val), val)
}

// drainFixture is a representative VSLRTH drain: job 100 (drop=2) has a complete mode-2
// RPC (seq 1), an unpaired/denied one (seq 2), and an empty-name one (seq 3); job 200
// is a names-only (mode 1) session with no drops.
func drainFixture() string {
	s := subSep
	return jhdr(100, "67751,40000-1", 1, 3, 2) +
		vnode(100, 1, "", "1^req^67751,40001^ORWU DT^2") +
		vnode(100, 1, "P"+s+"1", "lit") +
		vnode(100, 1, "R", "1") +
		vnode(100, 1, "R"+s+"1", "3070101") +
		vnode(100, 2, "", "1^req^67751,40002^XUS AV CODE^2") +
		vnode(100, 3, "", "1^req^67751,40003^^2") +
		jhdr(200, "67751,40000-2", 5, 5, 0) +
		vnode(200, 5, "", "1^req^67751,40005^ORWPT LIST^1")
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
	if r0.Job != 100 || r0.Seq != 1 || r0.Inc != "67751,40000-1" || r0.Head != 1 || r0.SeqMax != 3 || r0.Drop != 2 {
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
	if recs[3].Job != 200 || recs[3].Mode != 1 || recs[3].Class() != ClassNamesOnly || recs[3].Drop != 0 {
		t.Errorf("r3 should be names-only job 200 drop 0: %+v", recs[3])
	}
}

// The whole point of the length-prefixed format: a value carrying TABs, newlines, and
// even the byte sequence "\nV\t…" (a forged row prefix) is read by count, never
// mis-split — the failure mode the old line-based format had.
func TestParseDrainBinarySafeValue(t *testing.T) {
	hostile := "line-one\nV\t999\t1\t\tinjected\nline-three\twith\ttabs"
	in := vnode(100, 1, "R", hostile)
	recs, err := ParseDrain(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseDrain: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1 (the forged \\nV\\t prefix must NOT spawn a record)", len(recs))
	}
	if got := recs[0].Sub["R"]; got != hostile {
		t.Errorf("value not preserved verbatim:\n got %q\nwant %q", got, hostile)
	}
}

func TestParseDrainEmptyValue(t *testing.T) {
	in := vnode(100, 1, "", "1^req^H^RPC^2") + vnode(100, 1, "P"+subSep+"1", "")
	recs, err := ParseDrain(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseDrain: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if v, ok := recs[0].Sub["P"+subSep+"1"]; !ok || v != "" {
		t.Errorf("empty value node lost: ok=%v v=%q", ok, v)
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
		"V\t100\tNOTANUM\t\t3\nval\n", // seq not numeric
		"J\tNOTANUM\ti\t1\t3\t0\n",    // job not numeric
		"V\t100\t1\t\tNOTANUM\nval\n", // bytelen not numeric
		"V\tonly\ttwo\n",              // too few fields
	} {
		if _, err := ParseDrain(strings.NewReader(in)); err == nil {
			t.Errorf("ParseDrain(%q) = nil error, want a parse error", in)
		}
	}
}

// A J header may omit the trailing drop field (5 fields, older emitter) — the parser
// must tolerate it and default drop to 0, not error.
func TestParseDrainJHeaderBackwardCompatNoDrop(t *testing.T) {
	in := "J\t100\ti\t1\t1\n" + vnode(100, 1, "", "1^req^H^RPC^2")
	recs, err := ParseDrain(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseDrain: %v", err)
	}
	if len(recs) != 1 || recs[0].Drop != 0 {
		t.Errorf("want 1 record with drop 0, got %d records (%+v)", len(recs), recs)
	}
}
