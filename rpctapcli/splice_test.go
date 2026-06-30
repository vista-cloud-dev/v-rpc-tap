package rpctapcli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vista-cloud-dev/clikit"
)

// A minimal pinned CALLP^XWBPRS frame — enough for the verb to find both anchors.
const xwbprsFixture = `XWBPRS ;ISC/REL - Parse and execute RPC ;
 ;;1.1;RPC BROKER;**35,43,46,57,64,67**;Mar 28, 1997;Build 5
CALLP(XWBP,XWBDEBUG) ;make API call using Protocol string
 I '+ERR D CHKPRMIT^XWBSEC($G(XWB(2,"RPC")))
 S:$L($G(XWBSEC)) ERR="-1^"_XWBSEC
 I '+ERR D
 . D CAPI(.XWBP,XWB("PARAM"))
 Q
`

func TestSpliceVerb_WritesSplicedRoutine(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "XWBPRS.m")
	out := filepath.Join(dir, "out", "XWBPRS.m")
	if err := os.WriteFile(in, []byte(xwbprsFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}

	c := &spliceCmd{In: in, Out: out}
	if err := c.Run(jsonCtx(&bytes.Buffer{})); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, "\n D req^VSLRTAP\n") || !strings.Contains(s, "\n . D rsp^VSLRTAP\n") {
		t.Errorf("spliced output missing a tap call:\n%s", s)
	}
}

func TestSpliceVerb_RefusesNonXwbprs(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "OTHER.m")
	if err := os.WriteFile(in, []byte("OTHER ;not xwbprs\n Q\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := (&spliceCmd{In: in}).Run(jsonCtx(&bytes.Buffer{}))
	var ce *clikit.Error
	if !errors.As(err, &ce) || ce.Exit != clikit.ExitRuntime {
		t.Errorf("want a clikit ExitRuntime refusal, got %v", err)
	}
}

func TestSpliceVerb_MissingFileIsUsageError(t *testing.T) {
	err := (&spliceCmd{In: "/no/such/XWBPRS.m"}).Run(jsonCtx(&bytes.Buffer{}))
	var ce *clikit.Error
	if !errors.As(err, &ce) || ce.Exit != clikit.ExitUsage {
		t.Errorf("want a clikit ExitUsage error, got %v", err)
	}
}
