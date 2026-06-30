package splice

import (
	"strings"
	"testing"
)

// callpBlock is the pinned CALLP^XWBPRS frame (base **67, byte-identical on vehu
// and foia per the P0 seam pin). The two splice anchors are CALLP+9 (the XWBSEC
// denial line) and CALLP+11 (the dotted `D CAPI` line).
var callpBlock = []string{
	`XWBPRS ;ISC/REL,SFISC/RWF - Parse and execute RPC ;`,
	` ;;1.1;RPC BROKER;**35,43,46,57,64,67**;Mar 28, 1997;Build 5`,
	` ;`,
	`CALLP(XWBP,XWBDEBUG) ;make API call using Protocol string`,
	` N ERR,S,XWBARY K XWB`,
	` S ERR=0`,
	` S ERR=$$PRSP("[XWB]") ;Read the rest of the protocol header`,
	` I '+ERR S ERR=$$PRSM ;Read and parse message`,
	` I $G(XWB(2,"RPC"))="XUS SET SHARED" S XWBSHARE=1 Q`,
	` I '+ERR S ERR=$$RPC ;Check the RPC`,
	` I +ERR S XWBSEC=$P(ERR,U,2) ;P10 -- dpc`,
	` I '+ERR D CHKPRMIT^XWBSEC($G(XWB(2,"RPC"))) ;checks if RPC allowed to run`,
	` S:$L($G(XWBSEC)) ERR="-1^"_XWBSEC`,
	` I '+ERR D`,
	` . D CAPI(.XWBP,XWB("PARAM"))`,
	` E  I ($G(XWBTCMD)'="#BYE#") D LOG^XWBTCPM("Bad Msg"_ERR),CLRBUF`,
	` I 'XWBDEBUG K XWB`,
	` I $D(XWBARY) K @XWBARY,XWBARY`,
	` Q`,
	` ;`,
	`PRSP(P) ;ef, Parse Protocol`,
}

const (
	reqLine = ` D req^VSLRTAP`
	rspLine = ` . D rsp^VSLRTAP`
)

// indexOf returns the first index of the line whose trimmed text equals want, or -1.
func indexOf(lines []string, want string) int {
	for i, l := range lines {
		if strings.TrimSpace(l) == strings.TrimSpace(want) {
			return i
		}
	}
	return -1
}

func TestSplice_InsertsBothCallsAtPinnedAnchors(t *testing.T) {
	out, err := Splice(callpBlock)
	if err != nil {
		t.Fatalf("Splice: %v", err)
	}
	if len(out) != len(callpBlock)+2 {
		t.Fatalf("len(out) = %d, want %d (input + 2 spliced lines)", len(out), len(callpBlock)+2)
	}

	// req: the line immediately AFTER the XWBSEC denial line must be `D req^VSLRTAP`.
	denial := indexOf(out, `S:$L($G(XWBSEC)) ERR="-1^"_XWBSEC`)
	if denial < 0 {
		t.Fatal("XWBSEC denial anchor not found in output")
	}
	if got := out[denial+1]; got != reqLine {
		t.Errorf("line after XWBSEC denial = %q, want %q", got, reqLine)
	}

	// rsp: the line immediately AFTER `. D CAPI(...)` must be the dotted `. D rsp^VSLRTAP`.
	capi := indexOf(out, `. D CAPI(.XWBP,XWB("PARAM"))`)
	if capi < 0 {
		t.Fatal("D CAPI anchor not found in output")
	}
	if got := out[capi+1]; got != rspLine {
		t.Errorf("line after D CAPI = %q, want %q", got, rspLine)
	}

	// The rsp call must stay INSIDE the line-16 DO block — i.e. a dotted line.
	if !strings.HasPrefix(strings.TrimLeft(out[capi+1], " "), ".") {
		t.Errorf("rsp splice %q is not a dotted line (would fall outside the DO block)", out[capi+1])
	}
}

func TestSplice_PreservesEveryOtherLineInOrder(t *testing.T) {
	out, err := Splice(callpBlock)
	if err != nil {
		t.Fatalf("Splice: %v", err)
	}
	// Removing the two inserted lines must reproduce the input exactly (no other edits,
	// line 2 patch string untouched).
	var stripped []string
	for _, l := range out {
		if l == reqLine || l == rspLine {
			continue
		}
		stripped = append(stripped, l)
	}
	if strings.Join(stripped, "\n") != strings.Join(callpBlock, "\n") {
		t.Errorf("non-spliced lines changed:\n got:\n%s\nwant:\n%s",
			strings.Join(stripped, "\n"), strings.Join(callpBlock, "\n"))
	}
}

func TestSplice_RefusesIfAlreadySpliced(t *testing.T) {
	once, err := Splice(callpBlock)
	if err != nil {
		t.Fatalf("first Splice: %v", err)
	}
	if _, err := Splice(once); err == nil {
		t.Fatal("Splice of an already-spliced routine succeeded; want refusal (no double-splice)")
	}
}

func TestSplice_RefusesIfReqAnchorMissing(t *testing.T) {
	var noReq []string
	for _, l := range callpBlock {
		if strings.TrimSpace(l) == `S:$L($G(XWBSEC)) ERR="-1^"_XWBSEC` {
			continue
		}
		noReq = append(noReq, l)
	}
	if _, err := Splice(noReq); err == nil {
		t.Fatal("Splice with no XWBSEC anchor succeeded; want refusal")
	}
}

func TestSplice_RefusesIfCapiAnchorMissing(t *testing.T) {
	var noCapi []string
	for _, l := range callpBlock {
		if strings.TrimSpace(l) == `. D CAPI(.XWBP,XWB("PARAM"))` {
			continue
		}
		noCapi = append(noCapi, l)
	}
	if _, err := Splice(noCapi); err == nil {
		t.Fatal("Splice with no D CAPI anchor succeeded; want refusal")
	}
}

func TestSplice_RefusesIfNoCallpLabel(t *testing.T) {
	if _, err := Splice([]string{`XWBPRS ;header`, ` ;;1.1;RPC BROKER;**67**;`, ` Q`}); err == nil {
		t.Fatal("Splice with no CALLP label succeeded; want refusal")
	}
}
