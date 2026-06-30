package tap

import "sort"

// Session is one capture incarnation — all records sharing an (Inc, Job) — with its
// messages ordered by Seq. Two incarnations of the same $J (PID reuse across a reap)
// have distinct Inc and so never conflate.
type Session struct {
	Inc  string
	Job  int
	Msgs []Record
}

// Summary is the per-window accounting of a correlated drain.
type Summary struct {
	Sessions  int
	Records   int
	Complete  int
	Unpaired  int
	NamesOnly int
	EmptyName int
}

// Correlate groups records into sessions by (Inc, Job), de-duplicates re-drained
// records on Key (preferring the more complete copy — a later drain may have gained
// the result), and orders each session's messages by Seq. The L14 contract: unpaired
// and empty-name records are kept and classified, never dropped. Sessions are
// returned sorted by (Inc, Job) for determinism.
func Correlate(records []Record) []Session {
	best := make(map[string]Record, len(records))
	for _, r := range records {
		if cur, ok := best[r.Key()]; !ok || prefer(r, cur) {
			best[r.Key()] = r
		}
	}

	type groupKey struct {
		inc string
		job int
	}
	groups := map[groupKey][]Record{}
	for _, r := range best {
		k := groupKey{r.Inc, r.Job}
		groups[k] = append(groups[k], r)
	}

	out := make([]Session, 0, len(groups))
	for k, msgs := range groups {
		sort.Slice(msgs, func(i, j int) bool { return msgs[i].Seq < msgs[j].Seq })
		out = append(out, Session{Inc: k.inc, Job: k.job, Msgs: msgs})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Inc != out[j].Inc {
			return out[i].Inc < out[j].Inc
		}
		return out[i].Job < out[j].Job
	})
	return out
}

// prefer reports whether candidate should replace the incumbent for the same Key: a
// copy that carries a result wins (a later drain caught rsp@17.5); otherwise the one
// with more captured sub-nodes.
func prefer(candidate, incumbent Record) bool {
	if candidate.HasResult() != incumbent.HasResult() {
		return candidate.HasResult()
	}
	return len(candidate.Sub) > len(incumbent.Sub)
}

// Summarize tallies the per-window accounting across correlated sessions.
func Summarize(sessions []Session) Summary {
	s := Summary{Sessions: len(sessions)}
	for _, sess := range sessions {
		for _, m := range sess.Msgs {
			s.Records++
			switch m.Class() {
			case ClassComplete:
				s.Complete++
			case ClassUnpaired:
				s.Unpaired++
			case ClassNamesOnly:
				s.NamesOnly++
			}
			if !m.Named() {
				s.EmptyName++
			}
		}
	}
	return s
}
