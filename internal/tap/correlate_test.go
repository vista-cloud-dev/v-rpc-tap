package tap

import "testing"

func TestCorrelateGroupsOrdersAndClassifies(t *testing.T) {
	recs := []Record{
		{Inc: "i1", Job: 100, Seq: 1, Mode: 2, RPC: "A", Sub: map[string]string{"R": "1"}}, // complete
		{Inc: "i1", Job: 100, Seq: 3, Mode: 2, RPC: "B"},                                   // unpaired (out of order)
		{Inc: "i1", Job: 100, Seq: 2, Mode: 2, RPC: ""},                                    // unpaired + empty-name
		{Inc: "i2", Job: 200, Seq: 5, Mode: 1, RPC: "C"},                                   // names-only, other session
	}
	sessions := Correlate(recs)
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}

	// session order is deterministic by (Inc, Job); i1 before i2
	s0 := sessions[0]
	if s0.Inc != "i1" || s0.Job != 100 || len(s0.Msgs) != 3 {
		t.Fatalf("session 0 wrong: %+v", s0)
	}
	if s0.Msgs[0].Seq != 1 || s0.Msgs[1].Seq != 2 || s0.Msgs[2].Seq != 3 {
		t.Errorf("session 0 not ordered by seq: %d,%d,%d", s0.Msgs[0].Seq, s0.Msgs[1].Seq, s0.Msgs[2].Seq)
	}
	if sessions[1].Inc != "i2" || sessions[1].Job != 200 {
		t.Errorf("session 1 wrong: %+v", sessions[1])
	}

	got := Summarize(sessions)
	want := Summary{Sessions: 2, Records: 4, Complete: 1, Unpaired: 2, NamesOnly: 1, EmptyName: 1}
	if got != want {
		t.Errorf("Summarize = %+v, want %+v", got, want)
	}
}

func TestCorrelateDedupesReDrainPreferringComplete(t *testing.T) {
	reqOnly := Record{Inc: "i1", Job: 100, Seq: 1, Mode: 2, RPC: "A"}                                    // first drain (no result yet)
	complete := Record{Inc: "i1", Job: 100, Seq: 1, Mode: 2, RPC: "A", Sub: map[string]string{"R": "1"}} // re-drain after rsp

	for _, order := range [][]Record{{reqOnly, complete}, {complete, reqOnly}} {
		sessions := Correlate(order)
		if len(sessions) != 1 || len(sessions[0].Msgs) != 1 {
			t.Fatalf("dedup failed: %d sessions", len(sessions))
		}
		if !sessions[0].Msgs[0].HasResult() {
			t.Errorf("dedup kept the req-only copy, want the complete one")
		}
	}
}

func TestCorrelateSegmentsByIncarnation(t *testing.T) {
	// same $J reused across a reap -> distinct Inc -> two sessions, never conflated
	recs := []Record{
		{Inc: "iA", Job: 100, Seq: 1, Mode: 1, RPC: "A"},
		{Inc: "iB", Job: 100, Seq: 1, Mode: 1, RPC: "B"},
	}
	sessions := Correlate(recs)
	if len(sessions) != 2 {
		t.Fatalf("PID reuse conflated: got %d sessions, want 2", len(sessions))
	}
}
