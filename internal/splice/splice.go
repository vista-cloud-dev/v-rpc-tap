// Package splice generates the foreign-overwrite XWBPRS routine for the VSL RPC
// TAP KIDS build: it takes the site's CURRENT CALLP^XWBPRS source and inserts the
// two bare tap calls (D7 — no logic in the broker) at the pinned seam anchors.
//
// This is BUILD INPUT, not an installer. The spliced routine it returns is handed
// to `v pkg build` and installed/restored exclusively by v-pkg (the audited
// foreign-overwrite path with --auto-snapshot / uninstall --restore). The package
// never touches a live engine.
//
// The seam (P0 pin, base **67, byte-identical on vehu/foia):
//
//	CALLP+9 :  S:$L($G(XWBSEC)) ERR="-1^"_XWBSEC   → insert  D req^VSLRTAP  after
//	CALLP+11:  . D CAPI(.XWBP,XWB("PARAM"))        → insert  . D rsp^VSLRTAP after
//
// req fires once per RPC-bearing message before dispatch; rsp fires inside the
// line-16 DO block after the call completes. $IO is the live client socket at both
// points, so VSLRTAP emits zero I/O (CF4).
package splice

import (
	"fmt"
	"strings"
)

// Generate is the byte-level form of Splice for the build/CLI: it takes a routine
// source file's bytes (as pulled from the site over the driver), splices it, and
// returns the spliced file bytes. A single trailing newline is preserved (M
// routine files end in one); the line content is otherwise byte-faithful.
func Generate(src []byte) ([]byte, error) {
	s := string(src)
	trailingNL := strings.HasSuffix(s, "\n")
	s = strings.TrimSuffix(s, "\n")
	out, err := Splice(strings.Split(s, "\n"))
	if err != nil {
		return nil, err
	}
	res := strings.Join(out, "\n")
	if trailingNL {
		res += "\n"
	}
	return []byte(res), nil
}

// Splice inserts the two tap calls into a CALLP^XWBPRS source and returns the
// spliced source (one string per routine line, no trailing newlines; line 2 / the
// patch string is left untouched — versioning is the build's job). It refuses —
// rather than splice blind — unless a CALLP block is present and each anchor
// appears exactly once within it, and it refuses to double-splice a routine that
// already carries a tap call. Refusal is the safe default for a national,
// checksum-audited routine: a site whose XWBPRS does not match the pinned shape
// fails generation here (and the patch-pin gate refuses the install) rather than
// being silently mis-spliced.
func Splice(src []string) ([]string, error) {
	if i := findCall(src); i >= 0 {
		return nil, fmt.Errorf("splice: routine already carries a tap call at line %d (%q) — refusing to double-splice", i+1, strings.TrimSpace(src[i]))
	}
	lo, hi, ok := callpBounds(src)
	if !ok {
		return nil, fmt.Errorf("splice: no CALLP label found — not an XWBPRS routine")
	}
	reqAt, err := uniqueAnchor(src, lo, hi, isReqAnchor, `XWBSEC denial line (CALLP+9)`)
	if err != nil {
		return nil, err
	}
	rspAt, err := uniqueAnchor(src, lo, hi, isCapiAnchor, `dotted D CAPI line (CALLP+11)`)
	if err != nil {
		return nil, err
	}
	reqCall := indentOf(src[reqAt]) + "D req^VSLRTAP"
	rspCall := indentOf(src[rspAt]) + "D rsp^VSLRTAP"

	// Insert after the later anchor first so the earlier index stays valid.
	out := append([]string(nil), src...)
	if rspAt > reqAt {
		out = insertAfter(out, rspAt, rspCall)
		out = insertAfter(out, reqAt, reqCall)
	} else {
		out = insertAfter(out, reqAt, reqCall)
		out = insertAfter(out, rspAt, rspCall)
	}
	return out, nil
}

// callpBounds returns [labelLine, nextLabelLine) for the CALLP block — the lines
// from the column-1 `CALLP` label up to (but not including) the next column-1
// label. ok is false if no CALLP label is found.
func callpBounds(src []string) (lo, hi int, ok bool) {
	lo = -1
	for i, l := range src {
		if isLabelLine(l, "CALLP") {
			lo = i
			break
		}
	}
	if lo < 0 {
		return 0, 0, false
	}
	hi = len(src)
	for i := lo + 1; i < len(src); i++ {
		if isLabelLine(src[i], "") {
			hi = i
			break
		}
	}
	return lo, hi, true
}

// isLabelLine reports whether line is an M label line (a token starting in column
// 1). If name is non-empty, the label must be exactly that name (allowing a
// formal-parameter list or comment to follow, e.g. `CALLP(...)`).
func isLabelLine(line, name string) bool {
	if line == "" || line[0] == ' ' || line[0] == '\t' {
		return false
	}
	if name == "" {
		return true
	}
	rest := strings.TrimPrefix(line, name)
	return rest != line && (rest == "" || rest[0] == ' ' || rest[0] == '\t' || rest[0] == '(' || rest[0] == ';')
}

// isReqAnchor matches the XWBSEC denial line — the `ERR="-1^"_XWBSEC` assignment
// is unique within CALLP, so it survives trivial whitespace/comment drift.
func isReqAnchor(line string) bool { return strings.Contains(line, `ERR="-1^"_XWBSEC`) }

// isCapiAnchor matches the dotted `D CAPI` dispatch line inside the line-16 DO
// block (dotted so the rsp call lands at the same level, staying in the block).
func isCapiAnchor(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(line, " \t"), ".") && strings.Contains(line, "D CAPI")
}

// uniqueAnchor returns the single index in [lo,hi) for which match is true,
// erroring if the anchor is missing or appears more than once.
func uniqueAnchor(src []string, lo, hi int, match func(string) bool, desc string) (int, error) {
	found := -1
	for i := lo; i < hi; i++ {
		if match(src[i]) {
			if found >= 0 {
				return 0, fmt.Errorf("splice: %s anchor is ambiguous (matches CALLP lines %d and %d)", desc, found+1, i+1)
			}
			found = i
		}
	}
	if found < 0 {
		return 0, fmt.Errorf("splice: %s anchor not found in the CALLP block", desc)
	}
	return found, nil
}

// indentOf returns a line's leading line-level indentation — the run of spaces,
// tabs, and dots before the first command — so an inserted call inherits the
// exact depth of its anchor (top-level for req, dotted for rsp).
func indentOf(line string) string {
	n := 0
	for n < len(line) {
		c := line[n]
		if c != ' ' && c != '\t' && c != '.' {
			break
		}
		n++
	}
	return line[:n]
}

// findCall returns the index of the first line that already calls req^VSLRTAP or
// rsp^VSLRTAP, or -1.
func findCall(src []string) int {
	for i, l := range src {
		if strings.Contains(l, "req^VSLRTAP") || strings.Contains(l, "rsp^VSLRTAP") {
			return i
		}
	}
	return -1
}

// insertAfter returns lines with s inserted immediately after index i.
func insertAfter(lines []string, i int, s string) []string {
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:i+1]...)
	out = append(out, s)
	out = append(out, lines[i+1:]...)
	return out
}
