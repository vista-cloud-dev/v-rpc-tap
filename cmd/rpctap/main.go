// Command rpctap is the standalone `v rpc-tap` binary — the durable, scalable
// VistA RPC-broker tap. It mounts the same rpctapcli.Commands the `v` umbrella
// embeds (capture control, the load bench, and the splice build-input generator),
// so the domain can be driven on its own during development and CI. Engine-bound
// verbs reach a live engine only through the mdriver/v-pkg seam (waterline rule 3);
// the splice verb is pure build input and never touches an engine.
package main

import (
	"os"

	"github.com/vista-cloud-dev/clikit"
	"github.com/vista-cloud-dev/v-rpc-tap/rpctapcli"
)

// CLI is the root grammar: clikit globals plus the embedded rpc-tap verb set.
type CLI struct {
	clikit.Globals
	rpctapcli.Commands
}

func main() {
	cli := &CLI{}
	os.Exit(clikit.Run(
		"rpc-tap",
		"VSL RPC TAP — durable, scalable VistA RPC-broker tap (capture control · load bench · splice build).",
		cli, &cli.Globals,
	))
}
