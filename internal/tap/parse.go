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
// (length-prefixed, binary-safe; see VSLRTH.m) is:
//
//	J <TAB> job <TAB> inc <TAB> head <TAB> seqmax <TAB> drop   <LF>   — per-job header
//	V <TAB> job <TAB> seq <TAB> subpath <TAB> bytelen         <LF>   — one ring node header
//	<value: exactly bytelen bytes, may contain TAB or LF>     <LF>   — the node's value payload
//
// The J/V header lines carry only controlled tokens (integers, the inc token, and a
// $C(2)-joined subpath of controlled subscripts), so they are TAB-framed; the only
// arbitrary bytes — the captured value — are read BY COUNT (bytelen), so a value
// containing TAB/LF or even a forged "\nV\t…" row prefix can never be mis-split.
// subpath is the ring subscripts beyond (buf,job,seq); "" is the base record node.
// drop is the per-job cumulative drop count and may be absent on an older 5-field J
// header (defaults to 0). Records are returned sorted by (job, seq) for determinism.
func ParseDrain(rd io.Reader) ([]Record, error) {
	type hdr struct {
		inc                string
		head, seqmax, drop int
	}
	headers := map[int]hdr{}
	recs := map[string]*Record{}
	var order []string

	br := bufio.NewReader(rd)
	for {
		line, err := br.ReadString('\n')
		line = strings.TrimSuffix(line, "\n")
		if line == "" && err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read drain: %w", err)
		}

		switch {
		case strings.HasPrefix(line, "J\t"):
			f := strings.SplitN(line, "\t", 6)
			if len(f) < 5 {
				return nil, fmt.Errorf("malformed J line: %q", line)
			}
			job, e := strconv.Atoi(f[1])
			if e != nil {
				return nil, fmt.Errorf("J job %q: %w", f[1], e)
			}
			head, e := strconv.Atoi(f[3])
			if e != nil {
				return nil, fmt.Errorf("J head %q: %w", f[3], e)
			}
			seqmax, e := strconv.Atoi(f[4])
			if e != nil {
				return nil, fmt.Errorf("J seqmax %q: %w", f[4], e)
			}
			drop := 0
			if len(f) >= 6 { // the drop field is a backward-compatible append
				if drop, e = strconv.Atoi(f[5]); e != nil {
					return nil, fmt.Errorf("J drop %q: %w", f[5], e)
				}
			}
			headers[job] = hdr{inc: f[2], head: head, seqmax: seqmax, drop: drop}

		case strings.HasPrefix(line, "V\t"):
			f := strings.SplitN(line, "\t", 5)
			if len(f) < 5 {
				return nil, fmt.Errorf("malformed V line: %q", line)
			}
			job, e := strconv.Atoi(f[1])
			if e != nil {
				return nil, fmt.Errorf("V job %q: %w", f[1], e)
			}
			seq, e := strconv.Atoi(f[2])
			if e != nil {
				return nil, fmt.Errorf("V seq %q: %w", f[2], e)
			}
			n, e := strconv.Atoi(f[4])
			if e != nil {
				return nil, fmt.Errorf("V bytelen %q: %w", f[4], e)
			}
			value, e := readValue(br, n)
			if e != nil {
				return nil, fmt.Errorf("V value (job %d seq %d, %d bytes): %w", job, seq, n, e)
			}
			subpath := f[3]
			key := f[1] + "^" + f[2]
			rec, ok := recs[key]
			if !ok {
				h := headers[job]
				rec = &Record{Job: job, Seq: seq, Inc: h.inc, Head: h.head, SeqMax: h.seqmax, Drop: h.drop, Sub: map[string]string{}}
				recs[key] = rec
				order = append(order, key)
			}
			if subpath == "" {
				parseBase(rec, value)
			} else {
				rec.Sub[subpath] = value
			}

		default:
			// A blank or unrecognized line outside a value (the value is consumed by
			// count above, so it never reaches here) — ignore.
		}

		if err == io.EOF {
			break
		}
	}

	out := make([]Record, 0, len(order))
	for _, key := range order {
		out = append(out, *recs[key])
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Job != out[j].Job {
			return out[i].Job < out[j].Job
		}
		return out[i].Seq < out[j].Seq
	})
	return out, nil
}

// readValue reads exactly n value bytes from br, then consumes the single LF the
// emitter writes after the value. A trailing byte that is not the expected LF is
// pushed back so the main loop re-reads it (defensive against a truncated stream).
func readValue(br *bufio.Reader, n int) (string, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(br, buf); err != nil {
		return "", err
	}
	if b, err := br.ReadByte(); err == nil && b != '\n' {
		_ = br.UnreadByte()
	}
	return string(buf), nil
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
