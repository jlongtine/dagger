package task

import (
	"context"
	"fmt"
	"os"

	"cuelang.org/go/cue"
	"github.com/rs/zerolog/log"
	"go.dagger.io/dagger-classic/compiler"
	"go.dagger.io/dagger-classic/plancontext"
	"go.dagger.io/dagger-classic/solver"
)

func init() {
	// Register("ClientEnv", func() Task { return &clientEnvTask{} })
}

type clientEnvTask struct {
}

func (t clientEnvTask) Run(ctx context.Context, pctx *plancontext.Context, s *solver.Solver, v *compiler.Value) (*compiler.Value, error) {
	log.Ctx(ctx).Debug().Msg("loading environment variables")

	fields, err := v.Fields(cue.Optional(true))
	if err != nil {
		return nil, err
	}

	envs := make(map[string]interface{})
	for _, field := range fields {
		if field.Selector == cue.Str("$dagger") {
			continue
		}
		envvar := field.Label()
		// val, err := t.getEnv(envvar, field.Value, field.IsOptional, pctx, s)

		val, hasDefault := field.Value.Default()

		env, hasEnv := os.LookupEnv(envvar)
		if !hasEnv {
			if field.IsOptional || hasDefault {
				// Ignore unset var if it's optional
				return nil, nil
			}
			return nil, fmt.Errorf("environment variable %q not set", envvar)
		}

		if plancontext.IsSecretValue(val) {
			dgr := s.Client.Core().Host().Variable(envvar).Secret().ID()
			secret := pctx.Secrets.New(env)
			return secret.MarshalCUE(), nil
		}

		if !hasDefault && val.IsConcrete() {
			return nil, fmt.Errorf("%s: unexpected concrete value, please use a type or set a default", envvar)
		}

		k := val.IncompleteKind()
		if k == cue.StringKind {
			return env, nil
		}

		return nil, fmt.Errorf("%s: unsupported type %q", envvar, k)

		if err != nil {
			return nil, err
		}
		if val != nil {
			envs[envvar] = val
		}
	}

	return compiler.NewValue().FillFields(envs)
}

func (t clientEnvTask) getEnv(envvar string, v *compiler.Value, isOpt bool, pctx *plancontext.Context, s *solver.Solver) (interface{}, error) {
	// Resolve default in disjunction if a type hasn't been specified
	val, hasDefault := v.Default()

	env, hasEnv := os.LookupEnv(envvar)
	if !hasEnv {
		if isOpt || hasDefault {
			// Ignore unset var if it's optional
			return nil, nil
		}
		return nil, fmt.Errorf("environment variable %q not set", envvar)
	}

	if plancontext.IsSecretValue(val) {
		dgr := s.Client.Core().Host().Variable(envvar).Secret().ID()
		secret := pctx.Secrets.New(env)
		return secret.MarshalCUE(), nil
	}

	if !hasDefault && val.IsConcrete() {
		return nil, fmt.Errorf("%s: unexpected concrete value, please use a type or set a default", envvar)
	}

	k := val.IncompleteKind()
	if k == cue.StringKind {
		return env, nil
	}

	return nil, fmt.Errorf("%s: unsupported type %q", envvar, k)
}
