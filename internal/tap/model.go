// Package tap models the records the VSL RPC TAP captures and reconstructs the
// per-[XWB]-message correlation contract (L14) the host needs to turn a raw
// VSLRTH drain into ordered, de-duplicated, classified sessions.
//
// A captured record is ONE [XWB] message: the in-path splice fires once per
// RPC-bearing message at req@15.5 (writing the base record + params) and, in
// full-payload mode, again at rsp@17.5 (adding the result to the SAME ring seq).
// So a record is "complete" when it carries a result and "unpaired" when it does
// not (a permission-denied or errored RPC whose dispatch produced no result, or a
// names-only capture). The message name may be empty (the broker failed to parse
// it before the name was set). The host tolerates all of these — never dropping or
// erroring on them — and keys every record by the durable (inc, job, seq) triple.
package tap

import (
	"strconv"
	"strings"
)

// subSep joins the ring sub-node subscripts beyond (buf,job,seq) in the drain
// wire format — VSLRTH emits them $C(2)-joined.
const subSep = "\x02"

// Record is one captured ring node — one [XWB] message — keyed by (Inc, Job, Seq).
type Record struct {
	Job     int               // the broker $J that captured it
	Seq     int               // per-job ring sequence
	Inc     string            // per-incarnation token (the segmentation key; same $J reused → new Inc)
	Head    int               // job ring head at drain time
	SeqMax  int               // job ring seqmax at drain time
	Drop    int               // per-job cumulative drop count at drain time (from the J header; R20)
	Ver     int               // record schema version
	Kind    string            // capture kind ("req")
	Horolog string            // $H capture timestamp
	RPC     string            // RPC name ("" = pre-name parse failure / empty-name message)
	Mode    int               // 1 = names-only, 2 = full-payload
	Sub     map[string]string // sub-node subpath → value (params/result), verbatim, for the host
}

// MsgClass classifies a record by the per-message correlation contract (L14).
type MsgClass int

const (
	// ClassNamesOnly is a mode-1 capture: req-only by design (no result expected).
	ClassNamesOnly MsgClass = iota
	// ClassComplete is a mode-2 capture that carries a result (req + rsp both fired).
	ClassComplete
	// ClassUnpaired is a mode-2 capture with no result — a denied/errored RPC, or
	// one still in flight when drained. Tolerated, never dropped.
	ClassUnpaired
)

// String renders the class for logs/summaries.
func (c MsgClass) String() string {
	switch c {
	case ClassNamesOnly:
		return "names-only"
	case ClassComplete:
		return "complete"
	case ClassUnpaired:
		return "unpaired"
	default:
		return "unknown"
	}
}

// Key is the durable de-dup identity of a record: a record re-drained (at-least-once
// delivery) collapses on this triple.
func (r Record) Key() string {
	return r.Inc + "^" + strconv.Itoa(r.Job) + "^" + strconv.Itoa(r.Seq)
}

// Named reports whether the record carries an RPC name (empty-name records are the
// only "noise" that reaches the tap — TCPConnect/#BYE# never do).
func (r Record) Named() bool {
	return r.RPC != ""
}

// HasResult reports whether a result sub-node (rsp@17.5) was captured — the "R"
// type node or any "R"<sep>n data node.
func (r Record) HasResult() bool {
	if _, ok := r.Sub["R"]; ok {
		return true
	}
	for k := range r.Sub {
		if strings.HasPrefix(k, "R"+subSep) {
			return true
		}
	}
	return false
}

// Class applies the L14 correlation contract to a single record: mode-1 captures
// are names-only by design; a mode-2 capture is complete when it carries a result
// and unpaired (denied/errored/in-flight) when it does not.
func (r Record) Class() MsgClass {
	if r.Mode == 1 {
		return ClassNamesOnly
	}
	if r.HasResult() {
		return ClassComplete
	}
	return ClassUnpaired
}
