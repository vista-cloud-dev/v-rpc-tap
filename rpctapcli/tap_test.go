package rpctapcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/vista-cloud-dev/clikit"
	"github.com/vista-cloud-dev/v-rpc-tap/internal/host"
)

// fakeExecer is a no-engine stand-in for host.Execer: it records every command and
// returns scripted output, so the verb plumbing (command generation + result
// shaping) is verified without a live engine.
type fakeExecer struct {
	out  func(cmd string) (string, error)
	cmds []string
}

func (f *fakeExecer) Exec(_ context.Context, command string) (string, error) {
	f.cmds = append(f.cmds, command)
	if f.out != nil {
		return f.out(command)
	}
	return "", nil
}

// jsonCtx returns a Context that emits the JSON envelope to buf, so a verb's
// Result data can be inspected.
func jsonCtx(buf *bytes.Buffer) *clikit.Context {
	return &clikit.Context{Stdout: buf, Stderr: io.Discard, Format: clikit.FormatJSON, Command: "rpc-tap test"}
}

// decode unmarshals the clikit envelope's Data field into v.
func decode(t *testing.T, buf *bytes.Buffer, v any) {
	t.Helper()
	var env struct {
		OK   bool            `json:"ok"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("envelope unmarshal: %v — %s", err, buf.String())
	}
	if !env.OK {
		t.Fatalf("envelope not ok: %s", buf.String())
	}
	if err := json.Unmarshal(env.Data, v); err != nil {
		t.Fatalf("data unmarshal: %v — %s", err, env.Data)
	}
}

func TestArm_CommandAndResult(t *testing.T) {
	fe := &fakeExecer{}
	c := &armCmd{Mode: 2, TTL: 90, Dur: 0}
	var buf bytes.Buffer
	if err := c.exec(jsonCtx(&buf), host.New(fe)); err != nil {
		t.Fatalf("exec: %v", err)
	}
	if len(fe.cmds) != 1 || fe.cmds[0] != "do arm^VSLRTH(2,90,0)" {
		t.Fatalf("command = %v, want [do arm^VSLRTH(2,90,0)]", fe.cmds)
	}
	var got struct {
		Mode int `json:"mode"`
		TTL  int `json:"ttl"`
		Dur  int `json:"dur"`
	}
	decode(t, &buf, &got)
	if got.Mode != 2 || got.TTL != 90 || got.Dur != 0 {
		t.Errorf("result = %+v, want mode 2 ttl 90 dur 0", got)
	}
}

func TestDisarm_Command(t *testing.T) {
	fe := &fakeExecer{}
	c := &disarmCmd{}
	var buf bytes.Buffer
	if err := c.exec(jsonCtx(&buf), host.New(fe)); err != nil {
		t.Fatalf("exec: %v", err)
	}
	if len(fe.cmds) != 1 || fe.cmds[0] != "do disarm^VSLRTH()" {
		t.Fatalf("command = %v, want [do disarm^VSLRTH()]", fe.cmds)
	}
}

func TestStatus_ParsesAndShapes(t *testing.T) {
	fe := &fakeExecer{out: func(string) (string, error) {
		return "on=2^epoch=66123,45000^jobs=3^records=17", nil
	}}
	c := &statusCmd{}
	var buf bytes.Buffer
	if err := c.exec(jsonCtx(&buf), host.New(fe)); err != nil {
		t.Fatalf("exec: %v", err)
	}
	if fe.cmds[0] != "write $$status^VSLRTH()" {
		t.Fatalf("command = %q", fe.cmds[0])
	}
	var got struct {
		Armed   bool `json:"armed"`
		Mode    int  `json:"mode"`
		Jobs    int  `json:"jobs"`
		Records int  `json:"records"`
	}
	decode(t, &buf, &got)
	if !got.Armed || got.Mode != 2 || got.Jobs != 3 || got.Records != 17 {
		t.Errorf("result = %+v, want armed mode 2 jobs 3 records 17", got)
	}
}

func TestStatus_Disarmed(t *testing.T) {
	fe := &fakeExecer{out: func(string) (string, error) {
		return "on=^epoch=^jobs=0^records=0", nil
	}}
	var buf bytes.Buffer
	if err := (&statusCmd{}).exec(jsonCtx(&buf), host.New(fe)); err != nil {
		t.Fatalf("exec: %v", err)
	}
	var got struct {
		Armed bool `json:"armed"`
		Mode  int  `json:"mode"`
	}
	decode(t, &buf, &got)
	if got.Armed || got.Mode != 0 {
		t.Errorf("result = %+v, want disarmed mode 0", got)
	}
}

func TestDrain_CorrelatesAndSummarizes(t *testing.T) {
	// One complete mode-2 record (req base + R result) on job 123.
	drain := strings.Join([]string{
		"J\t123\tabc\t0\t1",
		"V\t123\t1\t\t1^req^66123,1^FOO\x02BAR\t2",
		"V\t123\t1\tR\tHELLO",
		"",
	}, "\n")
	fe := &fakeExecer{out: func(string) (string, error) { return drain, nil }}
	c := &drainCmd{Lo: 0, Hi: 0}
	var buf bytes.Buffer
	if err := c.exec(jsonCtx(&buf), host.New(fe)); err != nil {
		t.Fatalf("exec: %v", err)
	}
	if fe.cmds[0] != "do drain^VSLRTH(0,0)" {
		t.Fatalf("command = %q", fe.cmds[0])
	}
	var got struct {
		Sessions int `json:"sessions"`
		Records  int `json:"records"`
		Complete int `json:"complete"`
	}
	decode(t, &buf, &got)
	if got.Sessions != 1 || got.Records != 1 || got.Complete != 1 {
		t.Errorf("result = %+v, want 1 session / 1 record / 1 complete", got)
	}
}

func TestCommitTrim_Command(t *testing.T) {
	fe := &fakeExecer{}
	c := &commitTrimCmd{Job: 123, Seq: 7}
	var buf bytes.Buffer
	if err := c.exec(jsonCtx(&buf), host.New(fe)); err != nil {
		t.Fatalf("exec: %v", err)
	}
	if len(fe.cmds) != 1 || fe.cmds[0] != "do committrim^VSLRTH(123,7)" {
		t.Fatalf("command = %v, want [do committrim^VSLRTH(123,7)]", fe.cmds)
	}
}

// textCtx renders human output (the Result text closures) to buf.
func textCtx(buf *bytes.Buffer) *clikit.Context {
	return &clikit.Context{Stdout: buf, Stderr: io.Discard, Format: clikit.FormatText, Command: "rpc-tap test"}
}

// The human-render closures must emit a non-empty line carrying the engine name —
// exercises the text path the JSON tests skip.
func TestVerbs_TextRender(t *testing.T) {
	ydb := engineConn{Engine: "ydb"}
	cases := map[string]func(*clikit.Context, *host.Tap) error{
		"arm":        (&armCmd{engineConn: ydb, Mode: 2, TTL: 90}).exec,
		"disarm":     (&disarmCmd{engineConn: ydb}).exec,
		"committrim": (&commitTrimCmd{engineConn: ydb, Job: 1, Seq: 1}).exec,
	}
	for name, exec := range cases {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := exec(textCtx(&buf), host.New(&fakeExecer{})); err != nil {
				t.Fatalf("exec: %v", err)
			}
			if !strings.Contains(buf.String(), "ydb") {
				t.Errorf("text output %q missing engine name", buf.String())
			}
		})
	}
}

func TestStatusDrain_TextRender(t *testing.T) {
	var sb bytes.Buffer
	ydb := engineConn{Engine: "ydb"}
	stFake := &fakeExecer{out: func(string) (string, error) { return "on=2^epoch=1^jobs=1^records=1", nil }}
	if err := (&statusCmd{engineConn: ydb}).exec(textCtx(&sb), host.New(stFake)); err != nil {
		t.Fatalf("status exec: %v", err)
	}
	if !strings.Contains(sb.String(), "ON mode 2") {
		t.Errorf("status text %q missing armed marker", sb.String())
	}
	var db bytes.Buffer
	drFake := &fakeExecer{out: func(string) (string, error) { return "", nil }} // empty drain
	if err := (&drainCmd{engineConn: ydb}).exec(textCtx(&db), host.New(drFake)); err != nil {
		t.Fatalf("drain exec: %v", err)
	}
	if !strings.Contains(db.String(), "0 session(s)") {
		t.Errorf("drain text %q missing session count", db.String())
	}
}

// A propagated engine error from the controller becomes a clikit runtime failure,
// not a panic or a success envelope.
func TestArm_EngineErrorIsRuntimeFail(t *testing.T) {
	fe := &fakeExecer{out: func(string) (string, error) {
		return "", context.DeadlineExceeded
	}}
	err := (&armCmd{Mode: 2, TTL: 90}).exec(jsonCtx(&bytes.Buffer{}), host.New(fe))
	var ce *clikit.Error
	if !errors.As(err, &ce) {
		t.Fatalf("want a clikit.Error, got %T: %v", err, err)
	}
	if ce.Exit != clikit.ExitRuntime {
		t.Errorf("exit = %d, want ExitRuntime(%d)", ce.Exit, clikit.ExitRuntime)
	}
}
