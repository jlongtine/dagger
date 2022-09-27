package task

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/rs/zerolog/log"
	"go.dagger.io/dagger-classic/compiler"
	"go.dagger.io/dagger-classic/plancontext"
	"go.dagger.io/dagger-classic/solver"
)

func init() {
	// Register("ClientNetwork", func() Task { return &clientNetwork{} })
}

type clientNetwork struct {
}

func (t clientNetwork) Run(ctx context.Context, pctx *plancontext.Context, _ *solver.Solver, v *compiler.Value) (*compiler.Value, error) {
	lg := log.Ctx(ctx)

	addr, err := v.Lookup("address").String()
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	lg.Debug().Str("type", u.Scheme).Str("path", u.Path).Msg("loading local socket")

	if _, err := os.Stat(u.Path); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("path %q does not exist", u.Path)
	}

	var unix, npipe string

	switch u.Scheme {
	case "unix":
		unix = u.Path
	case "npipe":
		npipe = u.Path
	default:
		return nil, fmt.Errorf("invalid socket type %q", u.Scheme)
	}

	connect := v.Lookup("connect")

	if !plancontext.IsSocketValue(connect) {
		return nil, fmt.Errorf("wrong type %q", connect.Kind())
	}

	socket := pctx.Sockets.New(unix, npipe)

	return compiler.NewValue().FillFields(map[string]interface{}{
		"connect": socket.MarshalCUE(),
	})
}
