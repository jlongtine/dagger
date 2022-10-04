package task

import (
	"context"
	"fmt"

	"cuelang.org/go/cue"
	"github.com/Khan/genqlient/graphql"
	"go.dagger.io/dagger-classic/cloak/utils"
	"go.dagger.io/dagger-classic/compiler"
	"go.dagger.io/dagger-classic/plancontext"
	"go.dagger.io/dagger-classic/solver"
	"go.dagger.io/dagger/engine"
)

func init() {
	Register("ReadFile", func() Task { return &readFileTask{} })
}

type readFileTask struct {
}

func (t *readFileTask) Run(ctx context.Context, pctx *plancontext.Context, ectx *engine.Context, _ *solver.Solver, v *compiler.Value) (*compiler.Value, error) {
	path, err := v.Lookup("path").String()
	if err != nil {
		return nil, err
	}

	fsid, err := utils.GetFSId(v.Lookup("input"))

	if err != nil {
		return nil, err
	}

	res := struct {
		Core struct {
			Filesystem struct {
				File string
			}
		}
	}{}

	err = ectx.Client.MakeRequest(ctx,
		&graphql.Request{
			Query: `
			query ($fsid: FSID!, $path: String!) {
				core {
					filesystem(id: $fsid) {
						file(
							path: $path
						) 
					}
				}
			}
			`,
			Variables: &map[string]interface{}{
				"fsid": fsid,
				"path": path,
			},
		},
		&graphql.Response{Data: &res},
	)

	// FIXME: we should create an intermediate image containing only `path`.
	// That way, on cache misses, we'll only download the layer with the file contents rather than the entire FS.
	if err != nil {
		return nil, fmt.Errorf("ReadFile %s: %w", path, err)
	}

	output := compiler.NewValue()
	if err := output.FillPath(cue.ParsePath("contents"), string(res.Core.Filesystem.File)); err != nil {
		return nil, err
	}

	return output, nil
}
