package tap

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// ParseDrain parses a VSLRTH drain stream into records. The wire format
// (tab-delimited, \n-terminated; see VSLRTH.m) is:
//
//	J <TAB> job <TAB> inc <TAB> head <TAB> seqmax            — per-job header
//	V <TAB> job <TAB> seq <TAB> subpath <TAB> value          — one ring node
//
// subpath is the ring subscripts beyond (buf,job,seq), $C(2)-joined; "" is the base
// record node. A line that is neither a J nor a V row is treated as a continuation
// of the previous value (an embedded newline in a captured payload), so multi-line
// values survive. (A payload byte sequence "\nV\t…" or "\nJ\t…" remains ambiguous in
// this line-based format — a length-prefixed drain is the eventual hardening; for
// now the documented format is parsed as specified.) Records are returned sorted by
// (job, seq) for determinism.
func ParseDrain(rd io.Reader) ([]Record, error) {
	type hdr struct {
		inc          string
		head, seqmax int
	}
	headers := map[int]hdr{}
	recs := map[string]*Record{}
	baseRaw := map[string]string{}
	var order []string
	var lastKey, lastSub string
	haveLast := false

	sc := bufio.NewScanner(rd)
	sc.Buffer(make([]byte, 0, 64*1024), 64*1024*1024) // big captured payloads can make long lines

	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "J\t"):
			f := strings.SplitN(line, "\t", 5)
			if len(f) < 5 {
				return nil, fmt.Errorf("malformed J line: %q", line)
			}
			job, err := strconv.Atoi(f[1])
			if err != nil {
				return nil, fmt.Errorf("J job %q: %w", f[1], err)
			}
			head, err := strconv.Atoi(f[3])
			if err != nil {
				return nil, fmt.Errorf("J head %q: %w", f[3], err)
			}
			seqmax, err := strconv.Atoi(f[4])
			if err != nil {
				return nil, fmt.Errorf("J seqmax %q: %w", f[4], err)
			}
			headers[job] = hdr{inc: f[2], head: head, seqmax: seqmax}
			haveLast = false
		case strings.HasPrefix(line, "V\t"):
			f := strings.SplitN(line, "\t", 5)
			if len(f) < 5 {
				return nil, fmt.Errorf("malformed V line: %q", line)
			}
			job, err := strconv.Atoi(f[1])
			if err != nil {
				return nil, fmt.Errorf("V job %q: %w", f[1], err)
			}
			seq, err := strconv.Atoi(f[2])
			if err != nil {
				return nil, fmt.Errorf("V seq %q: %w", f[2], err)
			}
			subpath, value := f[3], f[4]
			key := f[1] + "^" + f[2]
			rec, ok := recs[key]
			if !ok {
				h := headers[job]
				rec = &Record{Job: job, Seq: seq, Inc: h.inc, Head: h.head, SeqMax: h.seqmax, Sub: map[string]string{}}
				recs[key] = rec
				order = append(order, key)
			}
			if subpath == "" {
				baseRaw[key] = value
			} else {
				rec.Sub[subpath] = value
			}
			lastKey, lastSub, haveLast = key, subpath, true
		default:
			if !haveLast {
				continue // stray content before any record row — ignore
			}
			if lastSub == "" {
				baseRaw[lastKey] += "\n" + line
			} else {
				recs[lastKey].Sub[lastSub] += "\n" + line
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan drain: %w", err)
	}

	out := make([]Record, 0, len(order))
	for _, key := range order {
		rec := recs[key]
		if raw, ok := baseRaw[key]; ok {
			parseBase(rec, raw)
		}
		out = append(out, *rec)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Job != out[j].Job {
			return out[i].Job < out[j].Job
		}
		return out[i].Seq < out[j].Seq
	})
	return out, nil
}

// parseBase fills the base record fields from the schema-v1 value
// "ver^kind^horolog^rpc^mode".
func parseBase(rec *Record, raw string) {
	f := strings.SplitN(raw, "^", 5)
	get := func(i int) string {
		if i < len(f) {
			return f[i]
		}
		return ""
	}
	rec.Ver, _ = strconv.Atoi(get(0))
	rec.Kind = get(1)
	rec.Horolog = get(2)
	rec.RPC = get(3)
	rec.Mode, _ = strconv.Atoi(get(4))
}
