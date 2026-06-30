package host

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeExecer records the M commands it is asked to run and replays canned output —
// no engine needed (house style: a fake, not a mock).
type fakeExecer struct {
	calls []string
	out   map[string]string
	err   error
}

func (f *fakeExecer) Exec(_ context.Context, command string) (string, error) {
	f.calls = append(f.calls, command)
	if f.err != nil {
		return "", f.err
	}
	return f.out[command], nil
}

func TestCommandsGenerateExpectedM(t *testing.T) {
	tests := []struct {
		name string
		run  func(context.Context, *Tap) error
		want string
	}{
		{"arm", func(c context.Context, tp *Tap) error { return tp.Arm(c, 2, 90, 0) }, "do arm^VSLRTH(2,90,0)"},
		{"disarm", func(c context.Context, tp *Tap) error { return tp.Disarm(c) }, "do disarm^VSLRTH()"},
		{"committrim", func(c context.Context, tp *Tap) error { return tp.CommitTrim(c, 100, 42) }, "do committrim^VSLRTH(100,42)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeExecer{}
			if err := tc.run(context.Background(), New(f)); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if len(f.calls) != 1 || f.calls[0] != tc.want {
				t.Errorf("got calls %v, want [%q]", f.calls, tc.want)
			}
		})
	}
}

func TestStatusParses(t *testing.T) {
	f := &fakeExecer{out: map[string]string{
		"write $$status^VSLRTH()": "on=2^epoch=67751,40000^jobs=3^records=42\n",
	}}
	st, err := New(f).Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := Status{On: "2", Epoch: "67751,40000", Jobs: 3, Records: 42}
	if st != want {
		t.Errorf("Status = %+v, want %+v", st, want)
	}
}

func TestStatusDisarmedEmptyOn(t *testing.T) {
	f := &fakeExecer{out: map[string]string{
		"write $$status^VSLRTH()": "on=^epoch=^jobs=0^records=0",
	}}
	st, err := New(f).Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.On != "" || st.Jobs != 0 || st.Records != 0 {
		t.Errorf("disarmed status parsed wrong: %+v", st)
	}
}

func TestDrainParsesAndCorrelates(t *testing.T) {
	drain := strings.Join([]string{
		"J\t100\t67751,40000-1\t1\t1",
		"V\t100\t1\t\t1^req^67751,40001^ORWU DT^1",
		"J\t200\t67751,40000-2\t1\t1",
		"V\t200\t1\t\t1^req^67751,40002^ORWPT LIST^1",
	}, "\n") + "\n"
	f := &fakeExecer{out: map[string]string{"do drain^VSLRTH(0,0)": drain}}
	sessions, err := New(f).Drain(context.Background(), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}
}

func TestEngineErrorPropagates(t *testing.T) {
	f := &fakeExecer{err: errors.New("engine fault")}
	tp := New(f)
	ctx := context.Background()
	if err := tp.Arm(ctx, 1, 0, 0); err == nil {
		t.Error("Arm should propagate the engine error")
	}
	if _, err := tp.Status(ctx); err == nil {
		t.Error("Status should propagate the engine error")
	}
	if _, err := tp.Drain(ctx, 0, 0); err == nil {
		t.Error("Drain should propagate the engine error")
	}
}
