package rpctapcli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/vista-cloud-dev/clikit"
	mdriver "github.com/vista-cloud-dev/m-driver-sdk"
	"github.com/vista-cloud-dev/v-rpc-tap/internal/host"
)

// engineConn selects which engine to drive and over which transport — the same
// neutral knobs as v-pkg/`m vista`/v-rpc-debug. The connection (container/base-url,
// credentials) is read by the driver from its M_<ENGINE>_* environment; the optional
// --container is a convenience that sets M_<ENGINE>_CONTAINER for this process. A
// distinct VRPCTAP_* env prefix keeps it from colliding with v-rpc-debug's VRPC_* when
// both domains are mounted under the same `v` umbrella process.
type engineConn struct {
	Engine    string `help:"Engine to reach: ydb or iris ($VRPCTAP_ENGINE)." enum:"ydb,iris" default:"ydb" env:"VRPCTAP_ENGINE"`
	Transport string `help:"Driver transport: local | docker | remote ($VRPCTAP_TRANSPORT)." enum:"local,docker,remote" default:"docker" env:"VRPCTAP_TRANSPORT"`
	Container string `help:"Engine container/instance name; sets M_<ENGINE>_CONTAINER ($VRPCTAP_CONTAINER)." placeholder:"NAME" env:"VRPCTAP_CONTAINER"`
}

// envContainer returns the M_<ENGINE>_CONTAINER value for engine.
func envContainer(engine string) string {
	return os.Getenv("M_" + strings.ToUpper(engine) + "_CONTAINER")
}

// execer resolves the m-<engine> driver (driver-contract §4) and returns the host's
// Execer seam backed by the shared reference Client — the single sanctioned transport
// (waterline rule 3). v-rpc-tap never hand-rolls transport.
func (e engineConn) execer() (host.Execer, *clikit.Error) {
	envKey := "M_" + strings.ToUpper(e.Engine) + "_CONTAINER"
	if e.Container != "" {
		_ = os.Setenv(envKey, e.Container)
	}
	// Docker transport execs M inside a named container, so the container name is the
	// one irreducible input (engine + transport both default). Surface its absence as a
	// USAGE error up front — not a later cryptic engine fault — so clikit answers a bare
	// verb with the verb's help.
	if e.Transport == "docker" && os.Getenv(envKey) == "" {
		return nil, clikit.Fail(clikit.ExitUsage, "USAGE",
			"no engine container: --engine "+e.Engine+" over docker transport needs a container name",
			"pass --container NAME or set $VRPCTAP_CONTAINER (e.g. vehu for ydb, foia-t12 for iris)")
	}
	bin, err := mdriver.Locate(e.Engine, mdriver.DefaultLocateDeps())
	if err != nil {
		return nil, clikit.Fail(clikit.ExitRefused, "NO_DRIVER", err.Error(),
			"build the m-"+e.Engine+" driver (make build) or set M_"+strings.ToUpper(e.Engine)+"_BIN")
	}
	cl := mdriver.NewClient(bin, e.Engine, e.Transport, nil, nil)
	return mdriverExecer{cl: cl}, nil
}

// mdriverExecer adapts mdriver.Client.ExecEval to host.Execer: a structured engine
// fault (EngineError) becomes a Go error so the command can report it.
type mdriverExecer struct{ cl *mdriver.Client }

func (m mdriverExecer) Exec(ctx context.Context, command string) (string, error) {
	res, err := m.cl.ExecEval(ctx, command)
	if err != nil {
		return "", err
	}
	if res.EngineError != nil {
		return "", fmt.Errorf("engine fault %s: %s", res.EngineError.Mnemonic, res.EngineError.Text)
	}
	return res.Stdout, nil
}
