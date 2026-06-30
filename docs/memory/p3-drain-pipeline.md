---
name: p3-drain-pipeline
description: The P3 read-only drain → object-store → committrim-after-ack pipeline (internal/pipeline) — the D12 at-least-once ack-gating, where the GovCloud egress pin (U4) is enforced, and what is deliberately deferred (the real AWS S3 sink + the CLI ship verb).
metadata:
  type: project
---

`internal/pipeline` is the read-only **drain → ship → committrim-after-ack** flow (D12),
pure Go, fully fake-tested (no engine, no AWS), gate-green (cover 90%). Durable points:

- **At-least-once ack-gating is the load-bearing invariant.** `RunOnce` drains, ships ONE
  LDJSON object per window to a `Sink`, and commits the engine-side trim **only after
  `Sink.Put` returns nil**. A failed Put trims **nothing** — the records stay in the ring
  for the next drain (de-duped on `(inc,job,seq)` by `tap.Correlate`). This is asserted by
  `TestRunOnce_PutFailureDoesNotTrim`; never reorder ship/trim.
- **committrim is per distinct job, to the drained `SeqMax`, in ascending job order** (one
  trim per job even across PID-reuse incarnations — `jobSeqMax` dedups by job). The host
  ring's `committrim(job,seq)` deletes `[head..seq]`.
- **The GovCloud egress pin (U4) lives in `pipeline.Config.Validate`, NOT at an AWS layer:**
  it requires `Partition=="aws-us-gov"` and a `us-gov-*` region, refusing a commercial
  (`aws`) / China (`aws-cn`) target at construction (`New`), so the inherited FedRAMP-High
  ATO boundary is enforced in code rather than discovered at deploy. RPC traffic (incl. PHI)
  may egress only to GovCloud.
- **Sinks:** `MemSink` (tests) + `FileSink` (local dir, usable now — also the local-MinIO
  lab stand-in) carry the whole flow. The **real S3 sink (aws-sdk-go + live creds) is
  DEFERRED**, as is the **CLI `ship`/`drain --to` verb** that would drive the pipeline — both
  ride a later slice (the verb can use `FileSink` for the lab and the GovCloud S3 sink for
  prod). LDJSON = one line per record (`lineRec`: inc/job/seq/rpc/mode/class/result/sub).

Drives the host controller through a `drainer` interface (`Drain`+`CommitTrim`) that
`*host.Tap` satisfies — same fakeable-seam pattern as the rest of the host. See
[[p3-host-l14]] for the host controller + CLI verbs + drain wire format this consumes,
and the central tracker P3 section.
