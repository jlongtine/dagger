package task

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/docker/distribution/reference"
	"go.dagger.io/dagger-classic/cloak/utils"
	"go.dagger.io/dagger-classic/compiler"
	"go.dagger.io/dagger-classic/plancontext"
	"go.dagger.io/dagger-classic/solver"
	"go.dagger.io/dagger/engine"
	"go.dagger.io/dagger/sdk/go/dagger"
)

func init() {
	Register("Pull", func() Task { return &pullTask{} })
}

type pullTask struct {
}

func (c *pullTask) Run(ctx context.Context, pctx *plancontext.Context, ectx *engine.Context, s *solver.Solver, v *compiler.Value) (*compiler.Value, error) {
	// lg := log.Ctx(ctx)

	rawRef, err := v.Lookup("source").String()
	if err != nil {
		return nil, err
	}

	// Extract registry target from source
	// target, err := solver.ParseAuthHost(rawRef)
	// if err != nil {
	// 	return nil, err
	// }

	// // Read auth info
	// if auth := v.Lookup("auth"); auth.Exists() {
	// 	a, err := decodeAuthValue(pctx, auth)
	// 	if err != nil {
	// 		return nil, err
	// 	}

	// 	s.AddCredentials(target, a.Username, a.Secret.PlainText())
	// 	lg.Debug().Str("target", target).Msg("add target credentials")
	// } else if target == "docker.io" {
	// 	// Collect DOCKERHUB_AUTH_USER && DOCKERHUB_AUTH_PASSWORD env vars
	// 	username, secret := "", ""
	// 	for _, envVar := range os.Environ() {
	// 		split := strings.SplitN(envVar, "=", 2)
	// 		if len(split) != 2 {
	// 			continue
	// 		}
	// 		key, val := split[0], split[1]
	// 		if strings.EqualFold(key, "dockerhub_auth_user") {
	// 			username = val
	// 		}
	// 		if strings.EqualFold(key, "dockerhub_auth_password") {
	// 			secret = val
	// 		}
	// 	}

	// 	if username != "" && secret != "" {
	// 		s.AddCredentials(target, username, secret)
	// 		lg.Debug().Str("target", target).Msg("add global credentials from DOCKERHUB_AUTH_USER and DOCKERHUB_AUTH_PASSWORD env vars")
	// 	}
	// }

	ref, err := reference.ParseNormalizedNamed(rawRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ref %s: %w", rawRef, err)
	}
	// Add the default tag "latest" to a reference if it only has a repo name.
	ref = reference.TagNameOnly(ref)

	fmt.Println("ref: ", ref)

	// var resolveMode llb.ResolveMode
	// resolveModeValue, err := v.Lookup("resolveMode").String()
	// if err != nil {
	// 	return nil, err
	// }

	// switch resolveModeValue {
	// case "default":
	// 	resolveMode = llb.ResolveModeDefault
	// case "forcePull":
	// 	resolveMode = llb.ResolveModeForcePull
	// case "preferLocal":
	// 	resolveMode = llb.ResolveModePreferLocal
	// default:
	// 	return nil, fmt.Errorf("unknown resolve mode for %s: %s", rawRef, resolveModeValue)
	// }

	// st := llb.Image(
	// 	ref.String(),
	// 	withCustomName(v, "Pull %s", rawRef),
	// 	resolveMode,
	// )

	// Load image metadata and convert to to LLB.
	// platform := pctx.Platform.Get()
	// image, digest, err := s.ResolveImageConfig(ctx, ref.String(), llb.ResolveImageConfigOpt{
	// 	LogName:     resolveImageConfigLogName(v, "load metadata for %s", ref.String()),
	// 	Platform:    &platform,
	// 	ResolveMode: resolveMode.String(),
	// })
	// if err != nil {
	// 	return nil, err
	// }

	// result, err := s.Solve(ctx, st, pctx.Platform.Get())
	// if err != nil {
	// 	return nil, err
	// }

	// resp := &graphql.Response{}

	// err = ectx.Client.MakeRequest(ctx, &graphql.Request{Query: `
	// query {
	// 		core {
	// 			image(ref: "alpine") {
	// 				exec(input: {
	// 					args: ["echo", "hello world"]
	// 					workdir: ""
	// 					env: [{
	// 						name: "JOEL"
	// 						value: "joel"
	// 					}]
	// 				}) {
	// 					exitCode
	// 					fs {
	// 						id
	// 					}
	// 					stderr
	// 					stdout
	// 				}
	// 			}
	// 		}
	// 	}
	// `}, resp)
	// fmt.Println("Query response:", resp.Data)

	res := struct {
		Core struct {
			Image struct {
				Id string
			}
		}
	}{}

	err = ectx.Client.MakeRequest(ctx,
		&graphql.Request{
			Query: `
			query ($ref: String!){
				core {
					image(ref: $ref) {
						id
					}
				}
			}
			`,
			Variables: &map[string]string{
				"ref": ref.String(),
			},
		},
		&graphql.Response{Data: &res},
	)

	// ir, err := core.Image(ectx, rawRef)
	fs := res.Core.Image

	// fs := pctx.FS.New(result)

	// return val, err
	return compiler.NewValue().FillFields(map[string]interface{}{
		"output": utils.NewFS(dagger.FSID(fs.Id)),
		// "digest": digest,
		// "config": ConvertImageConfig(image.Config),
	})
}
