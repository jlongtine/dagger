package task

import (
	"context"

	"go.dagger.io/dagger-classic/compiler"
	"go.dagger.io/dagger-classic/plancontext"
	"go.dagger.io/dagger-classic/solver"
	"go.dagger.io/dagger/engine"
)

func init() {
	// Register("Copy", func() Task { return &copyTask{} })
}

type copyTask struct {
}

func (t *copyTask) Run(ctx context.Context, pctx *plancontext.Context, ectx *engine.Context, s *solver.Solver, v *compiler.Value) (*compiler.Value, error) {
	var err error

	return nil, err

	// input, err := pctx.FS.FromValue(v.Lookup("input"))
	// if err != nil {
	// 	return nil, err
	// }

	// inputState, err := input.State()
	// if err != nil {
	// 	return nil, err
	// }

	// inputFsid, err := utils.GetFSId(v.Lookup("input"))

	// if err != nil {
	// 	return nil, err
	// }

	// contentsFsid, err := utils.GetFSId(v.Lookup("contents"))
	// if err != nil {
	// 	return nil, err
	// }

	// contentsState, err := contents.State()
	// if err != nil {
	// 	return nil, err
	// }

	// sourcePath, err := v.Lookup("source").String()
	// if err != nil {
	// 	return nil, err
	// }

	// destPath, err := v.Lookup("dest").String()
	// if err != nil {
	// 	return nil, err
	// }

	// var filters struct {
	// 	Include []string
	// 	Exclude []string
	// }

	// if err := v.Decode(&filters); err != nil {
	// 	return nil, err
	// }

	// // FIXME: allow more configurable llb options
	// // For now we define the following convenience presets.
	// // opts := &llb.CopyInfo{
	// // 	CopyDirContentsOnly: true,
	// // 	CreateDestPath:      true,
	// // 	AllowWildcard:       true,
	// // 	IncludePatterns:     filters.Include,
	// // 	ExcludePatterns:     filters.Exclude,
	// // }

	// // outputState := inputState.File(
	// // 	llb.Copy(
	// // 		contentsState,
	// // 		sourcePath,
	// // 		destPath,
	// // 		opts,
	// // 	),
	// // 	withCustomName(v, "Copy %s %s", sourcePath, destPath),
	// // )

	// // result, err := s.Solve(ctx, outputState, pctx.Platform.Get())
	// // if err != nil {
	// // 	return nil, err
	// // }

	// resp := &graphql.Response{
	// 	Data: &core.CacheMountInput,
	// }

	// ectx.Client.MakeRequest(ctx, &graphql.Request{}, resp *graphql.Response)

	// fs := pctx.FS.New(result)

	// return compiler.NewValue().FillFields(map[string]interface{}{
	// 	"output": fs.MarshalCUE(),
	// })
}
