package tap

import "testing"

func TestRecordClassAndName(t *testing.T) {
	tests := []struct {
		name      string
		rec       Record
		wantClass MsgClass
		wantNamed bool
		wantRes   bool
	}{
		{
			name:      "names-only mode 1",
			rec:       Record{Mode: 1, RPC: "ORWPT LIST"},
			wantClass: ClassNamesOnly, wantNamed: true, wantRes: false,
		},
		{
			name:      "complete mode 2 with result type node",
			rec:       Record{Mode: 2, RPC: "ORWU DT", Sub: map[string]string{"R": "1"}},
			wantClass: ClassComplete, wantNamed: true, wantRes: true,
		},
		{
			name:      "complete via result data node (R<sep>n)",
			rec:       Record{Mode: 2, RPC: "ORQQPL LIST", Sub: map[string]string{"R" + subSep + "1": "x"}},
			wantClass: ClassComplete, wantNamed: true, wantRes: true,
		},
		{
			name:      "unpaired mode 2 (denied/errored, no result)",
			rec:       Record{Mode: 2, RPC: "XUS AV CODE", Sub: map[string]string{"P" + subSep + "1": "lit"}},
			wantClass: ClassUnpaired, wantNamed: true, wantRes: false,
		},
		{
			name:      "empty-name mode 2 is unpaired and unnamed",
			rec:       Record{Mode: 2, RPC: ""},
			wantClass: ClassUnpaired, wantNamed: false, wantRes: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rec.Class(); got != tc.wantClass {
				t.Errorf("Class() = %v, want %v", got, tc.wantClass)
			}
			if got := tc.rec.Named(); got != tc.wantNamed {
				t.Errorf("Named() = %v, want %v", got, tc.wantNamed)
			}
			if got := tc.rec.HasResult(); got != tc.wantRes {
				t.Errorf("HasResult() = %v, want %v", got, tc.wantRes)
			}
		})
	}
}

func TestRecordKey(t *testing.T) {
	r := Record{Inc: "67751,40000-1", Job: 100, Seq: 3}
	if got, want := r.Key(), "67751,40000-1^100^3"; got != want {
		t.Errorf("Key() = %q, want %q", got, want)
	}
}
