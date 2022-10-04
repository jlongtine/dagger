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
	"go.dagger.io/dagger/sdk/go/dagger"
)

func init() {
	Register("WriteFile", func() Task { return &writeFileTask{} })
}

type writeFileTask struct {
}

func (t *writeFileTask) Run(ctx context.Context, pctx *plancontext.Context, ectx *engine.Context, s *solver.Solver, v *compiler.Value) (*compiler.Value, error) {
	var str string
	var err error

	path, err := v.Lookup("path").String()
	if err != nil {
		return nil, err
	}

	contentsVal := v.Lookup("contents")
	switch kind := contentsVal.Kind(); kind {
	// TODO: support bytes?
	// case cue.BytesKind:
	// 	contents, err = v.Lookup("contents").Bytes()
	case cue.StringKind:
		str, err = contentsVal.String()
	case cue.BottomKind:
		err = fmt.Errorf("%s: WriteFile contents is not set:\n\n%s", path, compiler.Err(contentsVal.Cue().Err()))
	default:
		err = fmt.Errorf("%s: unhandled data type in WriteFile: %s", path, kind)
	}

	if err != nil {
		return nil, err
	}

	// permissions, err := v.Lookup("permissions").Int64()
	// if err != nil {
	// 	return nil, err
	// }

	fsid, err := utils.GetFSId(v.Lookup("input"))

	if err != nil {
		return nil, err
	}

	res := struct {
		Core struct {
			Filesystem struct {
				WriteFile struct {
					ID string
				}
			}
		}
	}{}

	err = ectx.Client.MakeRequest(ctx,
		&graphql.Request{
			Query: `
			query ($fsid: FSID!, $contents: String!, $path: String!) {
				core {
					filesystem(id: $fsid) {
						writeFile(
							contents: $contents
							path: $path
						) {
							id
						}
					}
				}
			}
			`,
			Variables: &map[string]interface{}{
				"fsid":     fsid,
				"contents": str,
				"path":     path,
			},
		},
		&graphql.Response{Data: &res},
	)

	outputFS := utils.NewFS(dagger.FSID(res.Core.Filesystem.WriteFile.ID))

	return compiler.NewValue().FillFields(map[string]interface{}{
		"output": outputFS,
	})
}
