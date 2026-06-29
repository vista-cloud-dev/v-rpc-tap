// Package rpctapcli is the importable command surface of the `v rpc-tap`
// domain — the durable, scalable RPC-broker tap. The `v` umbrella mounts these
// structs as `v rpc-tap <verb>` (a sibling domain to v-rpc-debug's `rpc-debug`;
// the two integrate only at the v-cli busybox, never in one repo).
//
// Empty for now: this repo is the home for the scalable-tap effort (proposal
// v3.6.0, tracker in the shared docs repo). Verbs — arm / disarm / status /
// stream / drain / validate / doctor — land in P3 once the VSL RPC TAP M
// package (VSLRT* + KIDS build) and the P1/P2 safety+load proof are built. See
// docs/README.md for the effort's pointers.
package rpctapcli

// Commands is the `v rpc-tap` verb set, embedded by the umbrella and (later) the
// standalone binary. Add domain verbs as fields as P3 lands.
type Commands struct {
}
