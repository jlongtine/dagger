package task

import (
	"context"

	"github.com/Khan/genqlient/graphql"
	"go.dagger.io/dagger-classic/cloak/utils"
	"go.dagger.io/dagger-classic/compiler"
	"go.dagger.io/dagger-classic/plancontext"
	"go.dagger.io/dagger-classic/solver"
	"go.dagger.io/dagger/engine"
	"go.dagger.io/dagger/sdk/go/dagger"
)

func init() {
	Register("Copy", func() Task { return &copyTask{} })
}

type copyTask struct {
}

func (t *copyTask) Run(ctx context.Context, pctx *plancontext.Context, ectx *engine.Context, s *solver.Solver, v *compiler.Value) (*compiler.Value, error) {
	var err error

	// return nil, err

	// input, err := pctx.FS.FromValue(v.Lookup("input"))
	// if err != nil {
	// 	return nil, err
	// }

	// inputState, err := input.State()
	// if err != nil {
	// 	return nil, err
	// }

	inputFsid, err := utils.GetFSId(v.Lookup("input"))

	if err != nil {
		return nil, err
	}

	contentsFsid, err := utils.GetFSId(v.Lookup("contents"))
	if err != nil {
		return nil, err
	}

	// contentsState, err := contents.State()
	// if err != nil {
	// 	return nil, err
	// }

	sourcePath, err := v.Lookup("source").String()
	if err != nil {
		return nil, err
	}

	destPath, err := v.Lookup("dest").String()
	if err != nil {
		return nil, err
	}

	var filters struct {
		Include []string
		Exclude []string
	}

	if err := v.Decode(&filters); err != nil {
		return nil, err
	}

	// FIXME: allow more configurable llb options
	// For now we define the following convenience presets.
	// opts := &llb.CopyInfo{
	// 	CopyDirContentsOnly: true,
	// 	CreateDestPath:      true,
	// 	AllowWildcard:       true,
	// 	IncludePatterns:     filters.Include,
	// 	ExcludePatterns:     filters.Exclude,
	// }

	// outputState := inputState.File(
	// 	llb.Copy(
	// 		contentsState,
	// 		sourcePath,
	// 		destPath,
	// 		opts,
	// 	),
	// 	withCustomName(v, "Copy %s %s", sourcePath, destPath),
	// )

	// result, err := s.Solve(ctx, outputState, pctx.Platform.Get())
	// if err != nil {
	// 	return nil, err
	// }

	res := struct {
		Core struct {
			Filesystem struct {
				Copy struct {
					Id string
				}
			}
		}
	}{}

	err = ectx.Client.MakeRequest(ctx,
		&graphql.Request{
			Query: `
			query (
				$fsid: FSID!
				$from: FSID!
				$srcPath: String
				$destPath: String
				$include: [String!]
				$exclude: [String!]
			) {
				core {
					filesystem(id: $fsid) {
						copy (
							from: $from
							srcPath: $srcPath
							destPath: $destPath
							include: $include
							exclude: $exclude
						) {
							id
						}
					}
				}
			}
			`,
			Variables: &map[string]interface{}{
				"fsid":     inputFsid,
				"from":     contentsFsid,
				"srcPath":  sourcePath,
				"destPath": destPath,
				"include":  filters.Include,
				"exclude":  filters.Exclude,
			},
		},
		&graphql.Response{Data: &res},
	)

	fsid := res.Core.Filesystem.Copy.Id

	return compiler.NewValue().FillFields(map[string]interface{}{
		"output": pctx.FS.NewFS(dagger.FSID(fsid)),
	})
}
