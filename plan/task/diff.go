package task

import (
	"context"

	"go.dagger.io/dagger-classic/cloak/utils"
	"go.dagger.io/dagger-classic/compiler"
	"go.dagger.io/dagger-classic/plancontext"
	"go.dagger.io/dagger-classic/solver"
	"go.dagger.io/dagger/sdk/go/dagger"
	"go.dagger.io/dagger/sdk/go/dagger/api"
)

func init() {
	Register("Diff", func() Task { return &diffTask{} })
}

type diffTask struct {
}

func (t *diffTask) Run(ctx context.Context, pctx *plancontext.Context, s *solver.Solver, v *compiler.Value) (*compiler.Value, error) {
	dgr := s.Client.Core()

	lowerFSID, err := utils.GetFSId(v.Lookup("lower"))

	if err != nil {
		return nil, err
	}

	upperFSID, err := utils.GetFSId(v.Lookup("upper"))

	if err != nil {
		return nil, err
	}

	diffID, err := dgr.Directory(api.DirectoryOpts{ID: api.DirectoryID(lowerFSID)}).Diff(api.DirectoryID(upperFSID)).ID(ctx)
	if err != nil {
		return nil, err
	}

	return compiler.NewValue().FillFields(map[string]interface{}{
		"output": pctx.FS.NewFS(dagger.FSID(diffID)),
	})
}
