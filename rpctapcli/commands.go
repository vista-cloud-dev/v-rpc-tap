// Package rpctapcli is the importable command surface of the `v rpc-tap`
// domain — the durable, scalable RPC-broker tap. The `v` umbrella mounts these
// structs as `v rpc-tap <verb>` (a sibling domain to v-rpc-debug's `rpc-debug`;
// the two integrate only at the v-cli busybox, never in one repo).
//
// P3 control surface: the Control verbs (arm / disarm / status / drain /
// committrim) drive the engine-side host seam (VSLRTH) through the host.Tap
// controller over the mdriver-backed Execer (the single sanctioned transport,
// waterline rule 3). These are dep-wired + table-tested; their live smoke
// against an installed VSLRTH is DEFERRED, gated on the v-pkg install path.
// Still to land (each gated on the splice / load harness): stream / validate /
// doctor. See docs/README.md for the effort's pointers.
package rpctapcli

// Commands is the `v rpc-tap` verb set, embedded by the umbrella and (later) the
// standalone binary. Control drives the durable ring on a live engine; more verbs
// land as the splice + load proof come online.
type Commands struct {
	Arm        armCmd        `cmd:"" group:"Control" help:"Turn capture on at a mode with a host lease + absolute cap." example:"v rpc-tap arm --container vehu --mode 2"`
	Disarm     disarmCmd     `cmd:"" group:"Control" help:"Turn capture off (the reaper self-terminates on its next wake)." example:"v rpc-tap disarm --container vehu"`
	Status     statusCmd     `cmd:"" group:"Control" help:"Show the armed state + job ring / live record counts." example:"v rpc-tap status --container vehu"`
	Drain      drainCmd      `cmd:"" group:"Control" help:"Pull live records, correlate into sessions; deletes nothing (D12)." example:"v rpc-tap drain --container vehu"`
	CommitTrim commitTrimCmd `cmd:"" name:"committrim" group:"Control" help:"Trim the durable ring prefix after a drain is safe in S3 (at-least-once)." example:"v rpc-tap committrim --container vehu --job 123 --seq 42"`

	Load loadCmd `cmd:"" group:"Bench" help:"Drive concurrent [XWB] sessions at a broker and report throughput + latency (L8 load rig)." example:"v rpc-tap load --addr 127.0.0.1:9430 --concurrency 50 --total 500"`
}
