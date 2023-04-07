package core

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"dagger.io/dagger"
	"github.com/dagger/dagger/core/integration/internal"
	"github.com/moby/buildkit/identity"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func getDevEngine(ctx context.Context, c *dagger.Client, cache *dagger.Container, cacheName, cacheEnv string, index uint8) (devEngine *dagger.Container, endpoint string, err error) {
	id := identity.NewID()
	networkCIDR := fmt.Sprintf("10.%d.0.0/16", 89+index)
	opts := internal.DevEngineOpts{
		EntrypointArgs: map[string]string{
			"network-name": "dagger-dev",
			"network-cidr": networkCIDR,
		},
		ConfigEntries: map[string]string{
			"grpc":                     `address=["unix:///var/run/buildkit/buildkitd.sock", "tcp://0.0.0.0:1234"]`,
			`registry."docker.io"`:     `mirrors = ["mirror.gcr.io"]`,
			`registry."registry:5000"`: "http = true",
		},
	}
	devEngine = internal.DevEngineContainer(c.Pipeline("dagger-engine-"+id), []string{runtime.GOARCH}, opts)[0]
	devEngine = devEngine.
		WithServiceBinding(cacheName, cache).
		WithExposedPort(1234, dagger.ContainerWithExposedPortOpts{Protocol: dagger.Tcp}).
		WithEnvVariable("_EXPERIMENTAL_DAGGER_CACHE_CONFIG", cacheEnv).
		WithEnvVariable("ENGINE_INDEX", id).
		WithMountedCache("/var/lib/dagger", c.CacheVolume("dagger-dev-engine-state-"+id)).
		WithExec(nil, dagger.ContainerWithExecOpts{
			InsecureRootCapabilities:      true,
			ExperimentalPrivilegedNesting: true,
		})

	endpoint, err = devEngine.Endpoint(ctx, dagger.ContainerEndpointOpts{Port: 1234, Scheme: "tcp"})

	return devEngine, endpoint, err
}

func TestRemoteCacheRegistry(t *testing.T) {
	c, ctx := connect(t)
	defer c.Close()

	registry := c.Pipeline("registry").Container().From("registry:2").
		WithExposedPort(5000, dagger.ContainerWithExposedPortOpts{Protocol: dagger.Tcp}).
		WithExec(nil)

	devEngine, endpoint, err := getDevEngine(ctx, c, registry, "registry", "type=registry,ref=registry:5000/test-cache,mode=max", 0)
	require.NoError(t, err)

	cliBinPath := "/.dagger-cli"

	outputA, err := c.Container().From("alpine:3.17").
		WithServiceBinding("dev-engine", devEngine).
		WithMountedFile(cliBinPath, internal.DaggerBinary(c)).
		WithEnvVariable("_EXPERIMENTAL_DAGGER_CLI_BIN", cliBinPath).
		WithEnvVariable("_EXPERIMENTAL_DAGGER_RUNNER_HOST", endpoint).
		WithNewFile("/.dagger-query.txt", dagger.ContainerWithNewFileOpts{
			Contents: `{ 
				container { 
					from(address: "alpine:3.17") { 
						withExec(args: ["sh", "-c", "head -c 128 /dev/random | sha256sum"]) { 
							stdout 
						} 
					} 
				} 
			}`}).
		WithExec([]string{
			"sh", "-c", cliBinPath + ` query --doc .dagger-query.txt`,
		}).Stdout(ctx)
	require.NoError(t, err)
	shaA := strings.TrimSpace(gjson.Get(outputA, "container.from.exec.stdout").String())

	devEngine, endpoint, err = getDevEngine(ctx, c, registry, "registry", "type=registry,ref=registry:5000/test-cache,mode=max", 1)
	require.NoError(t, err)

	outputB, err := c.Container().From("alpine:3.17").
		WithServiceBinding("dev-engine", devEngine).
		WithMountedFile(cliBinPath, internal.DaggerBinary(c)).
		WithEnvVariable("_EXPERIMENTAL_DAGGER_CLI_BIN", cliBinPath).
		WithEnvVariable("_EXPERIMENTAL_DAGGER_RUNNER_HOST", endpoint).
		WithNewFile("/.dagger-query.txt", dagger.ContainerWithNewFileOpts{
			Contents: `{ 
				container { 
					from(address: "alpine:3.17") { 
						withExec(args: ["sh", "-c", "head -c 128 /dev/random | sha256sum"]) { 
							stdout 
						} 
					} 
				} 
			}`}).
		WithExec([]string{
			"sh", "-c", cliBinPath + " query --doc .dagger-query.txt",
		}).Stdout(ctx)
	require.NoError(t, err)
	shaB := strings.TrimSpace(gjson.Get(outputB, "container.from.exec.stdout").String())

	require.Equal(t, shaA, shaB)
}

func TestRemoteCacheS3(t *testing.T) {
	t.Run("buildkit s3 caching", func(t *testing.T) {
		c, ctx := connect(t)
		defer c.Close()

		bucket := "dagger-test-remote-cache-s3-" + identity.NewID()

		s3 := c.Pipeline("s3").Container().From("minio/minio").
			WithExposedPort(9000, dagger.ContainerWithExposedPortOpts{Protocol: dagger.Tcp}).
			WithExec([]string{"server", "/data"})

		minioStdout, err := c.Container().From("minio/mc").
			WithServiceBinding("s3", s3).
			WithEntrypoint([]string{"sh"}).
			WithExec([]string{"-c", "mc config host add minio http://s3:9000 minioadmin minioadmin && mc mb minio/" + bucket}).
			Stdout(ctx)
		require.NoError(t, err)
		fmt.Println(minioStdout)
		time.Sleep(1 * time.Second)

		s3Env := "type=s3,mode=max,endpoint_url=http://s3:9000,access_key_id=minioadmin,secret_access_key=minioadmin,region=mars,use_path_style=true,bucket=" + bucket

		devEngine, endpoint, err := getDevEngine(ctx, c, s3, "s3", s3Env, 0)
		require.NoError(t, err)

		cliBinPath := "/.dagger-cli"

		outputA, err := c.Container().From("alpine:3.17").
			WithServiceBinding("dev-engine", devEngine).
			WithMountedFile(cliBinPath, internal.DaggerBinary(c)).
			WithEnvVariable("_EXPERIMENTAL_DAGGER_CLI_BIN", cliBinPath).
			WithEnvVariable("_EXPERIMENTAL_DAGGER_RUNNER_HOST", endpoint).
			WithNewFile("/.dagger-query.txt", dagger.ContainerWithNewFileOpts{
				Contents: `{ 
						container { 
							from(address: "alpine:3.17") { 
								withExec(args: ["sh", "-c", "head -c 128 /dev/random | sha256sum"]) { 
									stdout 
								} 
							} 
						} 
					}`}).
			WithExec([]string{
				"sh", "-c", cliBinPath + ` query --doc .dagger-query.txt`,
			}).Stdout(ctx)
		require.NoError(t, err)
		shaA := strings.TrimSpace(gjson.Get(outputA, "container.from.exec.stdout").String())

		devEngine, endpoint, err = getDevEngine(ctx, c, s3, "s3", s3Env, 1)
		require.NoError(t, err)

		outputB, err := c.Container().From("alpine:3.17").
			WithServiceBinding("dev-engine", devEngine).
			WithMountedFile(cliBinPath, internal.DaggerBinary(c)).
			WithEnvVariable("_EXPERIMENTAL_DAGGER_CLI_BIN", cliBinPath).
			WithEnvVariable("_EXPERIMENTAL_DAGGER_RUNNER_HOST", endpoint).
			WithNewFile("/.dagger-query.txt", dagger.ContainerWithNewFileOpts{
				Contents: `{ 
						container { 
							from(address: "alpine:3.17") { 
								withExec(args: ["sh", "-c", "head -c 128 /dev/random | sha256sum"]) { 
									stdout 
								} 
							} 
						} 
					}`}).
			WithExec([]string{
				"sh", "-c", cliBinPath + " query --doc .dagger-query.txt",
			}).Stdout(ctx)
		require.NoError(t, err)
		shaB := strings.TrimSpace(gjson.Get(outputB, "container.from.exec.stdout").String())

		require.Equal(t, shaA, shaB)
	})
}
