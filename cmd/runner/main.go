package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/unix"

	bkclient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	bkgw "github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/progress/progressui"
)

const (
	buildkitdBin  = "/usr/bin/buildkitd"
	buildkitdSock = "/run/buildkit/buildkitd.sock"
)

func main() {
	if rc := run(); rc != 0 {
		os.Exit(rc)
	}
}

func exitError(err error, rc int) int {
	if err != nil {
		os.Stderr.WriteString(err.Error())
	}
	return rc
}

func run() int {
	cmd := exec.Command(buildkitdBin, os.Args[1:]...)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	sigCh := make(chan os.Signal, 32)
	signal.Notify(sigCh)
	if err := cmd.Start(); err != nil {
		return exitError(err, 1)
	}
	go func() {
		for sig := range sigCh {
			cmd.Process.Signal(sig)
		}
	}()

	eg, ctx := errgroup.WithContext(context.Background())
	ctx, cancel := context.WithCancel(ctx)

	var exitCode int
	eg.Go(func() error {
		defer cancel()
		err := cmd.Wait()
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		return err
	})

	progressCh := make(chan *bkclient.SolveStatus)
	eg.Go(func() error {
		warn, err := progressui.DisplaySolveStatus(context.TODO(), "", nil, os.Stdout, progressCh)
		for _, w := range warn {
			fmt.Fprintf(os.Stderr, "=> %s\n", w.Short)
		}
		return err
	})

	eg.Go(func() error {
		// TODO: retry logic instead of sleep
		time.Sleep(5 * time.Second)

		c, err := bkclient.New(ctx, "unix://"+buildkitdSock)
		if err != nil {
			return err
		}
		defer c.Close()

		// TODO: secrets provider, registry auth?, etc.
		solveOpts := bkclient.SolveOpt{}

		_, err = c.Build(ctx, solveOpts, "", func(ctx context.Context, gw bkgw.Client) (*bkgw.Result, error) {
			img := llb.Image("myoung34/github-runner:latest", llb.WithMetaResolver(gw))
			imgEnv, err := img.Env(ctx)
			if err != nil {
				return nil, err
			}
			if _, ok := os.LookupEnv("GHA_ACTIONS_TOKEN"); !ok {
				fmt.Println("GHA_ACTIONS_TOKEN not set")
				return nil, fmt.Errorf("GHA_ACTIONS_TOKEN is not set")
			}
			if _, ok := os.LookupEnv("GHA_ACTIONS_REPO"); !ok {
				fmt.Println("GHA_ACTIONS_REPO not set")
				return nil, fmt.Errorf("GHA_ACTIONS_REPO is not set")
			}
			imgEnv = append(imgEnv, "ACCESS_TOKEN="+os.Getenv("GHA_ACTIONS_TOKEN"))
			imgEnv = append(imgEnv, "REPO_URL="+os.Getenv("GHA_ACTIONS_REPO"))
			imgEnv = append(imgEnv, "LABELS="+"dagger-runner")
			imgEnv = append(imgEnv, "RUNNER_NAME="+"test-dagger-runner")
			imgEnv = append(imgEnv, "_DAGGER_ENABLE_NESTING=1")

			def, err := img.Marshal(ctx)
			if err != nil {
				return nil, err
			}
			res, err := gw.Solve(ctx, bkgw.SolveRequest{
				Evaluate:   true,
				Definition: def.ToPB(),
			})
			if err != nil {
				return nil, err
			}
			ref, err := res.SingleRef()
			if err != nil {
				return nil, err
			}

			rootMnt := bkgw.Mount{
				Dest: "/",
				Ref:  ref,
			}
			metaMount := bkgw.Mount{
				Dest:      "/.dagger_meta_mount",
				MountType: pb.MountType_TMPFS,
			}
			ctr, err := gw.NewContainer(ctx, bkgw.NewContainerRequest{
				Mounts: []bkgw.Mount{rootMnt, metaMount},
			})
			if err != nil {
				return nil, err
			}
			defer ctr.Release(ctx)

			proc, err := ctr.Start(ctx, bkgw.StartRequest{
				Args: []string{"/entrypoint.sh", "./bin/Runner.Listener", "run", "--startuptype", "service"},
				/*
					Args: []string{"sh", "-c",
						"env && /entrypoint.sh ./bin/Runner.Listener run --startuptype service",
					},
				*/
				Env:    imgEnv,
				Cwd:    "/actions-runner",
				Stdout: os.Stdout,
				Stderr: os.Stderr,
			})
			if err != nil {
				fmt.Println("start error", err)
				return nil, err
			}
			go func() {
				<-ctx.Done()
				proc.Signal(ctx, unix.SIGKILL)
				// TODO: technically signal could fail i guess
			}()
			if err := proc.Wait(); err != nil {
				fmt.Println("wait error", err)
				return nil, err
			}
			return nil, nil
		}, progressCh)
		if err != nil {
			fmt.Println("build error", err)
			return err
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return exitError(err, exitCode)
	}
	return exitCode
}
