package rpctapcli

import (
	"os"

	"github.com/vista-cloud-dev/clikit"
	"github.com/vista-cloud-dev/v-rpc-tap/internal/splice"
)

// The `v rpc-tap splice` verb regenerates the foreign-overwrite XWBPRS routine for
// the KIDS build from the SITE's CURRENT XWBPRS source (proposal §12: "not a frozen
// copy"). The operator pulls the live XWBPRS over the driver (read-only —
// `m sync diff/pull`), feeds it here, and the spliced output is what `v pkg build`
// ships and `v pkg` installs/restores. This verb NEVER touches a live engine and
// NEVER installs anything — it only produces build input. Refusal (a non-pinned
// XWBPRS) is a hard error, never a blind splice.

type spliceCmd struct {
	In  string `help:"Path to the site's current XWBPRS.m source (pull it over the driver first — read-only)." required:"" type:"existingfile"`
	Out string `help:"Write the spliced XWBPRS.m here (default: stdout)." type:"path"`
}

func (c *spliceCmd) Run(cc *clikit.Context) error {
	src, err := os.ReadFile(c.In)
	if err != nil {
		return clikit.Fail(clikit.ExitUsage, "READ", err.Error(), "")
	}
	out, err := splice.Generate(src)
	if err != nil {
		return clikit.Fail(clikit.ExitRuntime, "SPLICE", err.Error(),
			"the input must be the pinned CALLP^XWBPRS routine (regenerate from the target site's current XWBPRS)")
	}
	if c.Out == "" {
		_, werr := cc.Stdout.Write(out)
		return werr
	}
	if err := os.WriteFile(c.Out, out, 0o644); err != nil {
		return clikit.Fail(clikit.ExitRuntime, "WRITE", err.Error(), "")
	}
	return cc.Result(
		struct {
			In  string `json:"in"`
			Out string `json:"out"`
		}{c.In, c.Out},
		func() { /* text mode: silent on success, the file is the artifact */ },
	)
}
