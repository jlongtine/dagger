package task

import (
	"context"

	"go.dagger.io/dagger-classic/cloak/utils"
	"go.dagger.io/dagger-classic/compiler"
	"go.dagger.io/dagger-classic/plancontext"
	"go.dagger.io/dagger-classic/solver"
	"go.dagger.io/dagger/sdk/go/dagger/api"
)

func init() {
	// Register("NewSecret", func() Task { return &newSecretTask{} })
}

type newSecretTask struct {
}

func (t *newSecretTask) Run(ctx context.Context, pctx *plancontext.Context, s *solver.Solver, v *compiler.Value) (*compiler.Value, error) {
	path, err := v.Lookup("path").String()
	if err != nil {
		return nil, err
	}

	fsid, err := utils.GetFSId(v.Lookup("input"))

	if err != nil {
		return nil, err
	}

	dgr := s.Client.Core()

	secretid, err := dgr.Directory(api.DirectoryOpts{ID: api.DirectoryID(fsid)}).File(path).Secret().ID(ctx)

	// input, err := pctx.FS.FromValue(v.Lookup("input"))
	// if err != nil {
	// 	return nil, err
	// }
	// inputFS := solver.NewBuildkitFS(input.Result())

	// FIXME: we should create an intermediate image containing only `path`.
	// That way, on cache misses, we'll only download the layer with the file contents rather than the entire FS.
	// contents, err := fs.ReadFile(inputFS, path)
	// if err != nil {
	// 	return nil, fmt.Errorf("ReadFile %s: %w", path, err)
	// }
	// plaintext := string(contents)

	// trimSpace, err := v.Lookup("trimSpace").Bool()
	// if err != nil {
	// 	return nil, err
	// }
	// if trimSpace {
	// 	plaintext = strings.TrimSpace(plaintext)
	// }

	// secret := pctx.Secrets.New(plaintext)

	secret := pctx.Secrets.NewFromID(secretid)

	return compiler.NewValue().FillFields(map[string]interface{}{
		"output": secret.MarshalCUE(),
	})
}
