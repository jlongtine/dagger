package task

import (
	"context"

	"go.dagger.io/dagger-classic/compiler"
	"go.dagger.io/dagger-classic/plancontext"
	"go.dagger.io/dagger-classic/solver"
)

func init() {
	// Register("Nop", func() Task { return &nopTask{} })
}

type nopTask struct {
}

func (t *nopTask) Run(_ context.Context, _ *plancontext.Context, _ *solver.Solver, v *compiler.Value) (*compiler.Value, error) {
	return compiler.NewValue().FillFields(map[string]interface{}{
		"output": v.Lookup("input"),
	})
}
