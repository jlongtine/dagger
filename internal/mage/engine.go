package mage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dagger.io/dagger"
	"github.com/dagger/dagger/internal/mage/sdk"
	"github.com/dagger/dagger/internal/mage/util"
	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
	"golang.org/x/mod/semver"
)

const (
	engineBinName = "dagger-engine"
	shimBinName   = "dagger-shim"
	alpineVersion = "3.17"
	runcVersion   = "v1.1.5"
	qemuBinImage  = "tonistiigi/binfmt:buildkit-v7.1.0-30@sha256:45dd57b4ba2f24e2354f71f1e4e51f073cb7a28fd848ce6f5f2a7701142a6bf0"

	engineTomlPath = "/etc/dagger/engine.toml"
	// NOTE: this needs to be consistent with DefaultStateDir in internal/engine/docker.go
	engineDefaultStateDir = "/var/lib/dagger"

	engineEntrypointPath = "/usr/local/bin/dagger-entrypoint.sh"

	cacheConfigEnvName = "_EXPERIMENTAL_DAGGER_CACHE_CONFIG"
	servicesDNSEnvName = "_EXPERIMENTAL_DAGGER_SERVICES_DNS"
)

var engineEntrypoint string

// const engineEntrypointTmpl = `#!/bin/sh
// set -e

// # cgroup v2: enable nesting
// # see https://github.com/moby/moby/blob/38805f20f9bcc5e87869d6c79d432b166e1c88b4/hack/dind#L28
// if [ -f /sys/fs/cgroup/cgroup.controllers ]; then
// 	# move the processes from the root group to the /init group,
// 	# otherwise writing subtree_control fails with EBUSY.
// 	# An error during moving non-existent process (i.e., "cat") is ignored.
// 	mkdir -p /sys/fs/cgroup/init
// 	xargs -rn1 < /sys/fs/cgroup/cgroup.procs > /sys/fs/cgroup/init/cgroup.procs || :
// 	# enable controllers
// 	sed -e 's/ / +/g' -e 's/^/+/' < /sys/fs/cgroup/cgroup.controllers \
// 		> /sys/fs/cgroup/cgroup.subtree_control
// fi

// exec {{.EngineBin}} --config {{.EngineConfig}} {{ range $key := .EntrypointArgKeys -}}--{{ $key }}="{{ index $.EntrypointArgs $key }}" {{ end -}} "$@"
// `

// exec {{.EngineBin}} --config {{.EngineConfig}} --network-name dagger-dev --network-cidr "10.88.0.0/16" "$@"
// const engineConfig = `
// debug = true
// insecure-entitlements = ["security.insecure"]

// [grpc]
// address=["unix:///var/run/buildkit/buildkitd.sock", "tcp://0.0.0.0:1234"]

// [registry."registry:5000"]
// http = true

// [registry."privateregistry:5000"]
// http = true

// [registry."docker.io"]
// mirrors = ["mirror.gcr.io"]
// `

const engineConfigTmpl = `
debug = true
insecure-entitlements = ["security.insecure"]
{{ range $key := .ConfigKeys }}
[{{ $key }}]
{{ index $.ConfigEntries $key }}
{{ end -}}
`

// func init() {
// 	type entrypointTmplParams struct {
// 		Bridge       string
// 		EngineBin    string
// 		EngineConfig string
// 	}
// 	tmpl := template.Must(template.New("entrypoint").Parse(engineEntrypointTmpl))
// 	buf := new(bytes.Buffer)
// 	err := tmpl.Execute(buf, entrypointTmplParams{
// 		EngineBin:    "/usr/local/bin/" + engineBinName,
// 		EngineConfig: engineTomlPath,
// 	})
// 	if err != nil {
// 		panic(err)
// 	}
// 	engineEntrypoint = buf.String()
// }

// var engineToml = fmt.Sprintf(`root = %q
// `, engineDefaultStateDir)

var publishedEngineArches = []string{"amd64", "arm64"}

func parseRef(tag string) error {
	if tag == "main" {
		return nil
	}
	if ok := semver.IsValid(tag); !ok {
		return fmt.Errorf("invalid semver tag: %s", tag)
	}
	return nil
}

type Engine mg.Namespace

// Build builds the engine binary
func (t Engine) Build(ctx context.Context) error {
	c, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return err
	}
	defer c.Close()
	c = c.Pipeline("engine").Pipeline("build")

	_, err = util.HostDaggerBinary(c).Export(ctx, "./bin/dagger")

	return err
}

// Lint lints the engine
func (t Engine) Lint(ctx context.Context) error {
	c, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return err
	}
	defer c.Close()

	c = c.Pipeline("engine").Pipeline("lint")

	repo := util.RepositoryGoCodeOnly(c)

	_, err = c.Container().
		From("golangci/golangci-lint:v1.51").
		WithMountedDirectory("/app", repo).
		WithWorkdir("/app").
		WithExec([]string{"golangci-lint", "run", "-v", "--timeout", "5m"}).
		ExitCode(ctx)
	return err
}

// Publish builds and pushes Engine OCI image to a container registry
func (t Engine) Publish(ctx context.Context, version string) error {
	if err := parseRef(version); err != nil {
		return err
	}

	c, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return err
	}
	defer c.Close()

	c = c.Pipeline("engine").Pipeline("publish")
	engineImage, err := util.WithSetHostVar(ctx, c.Host(), "DAGGER_ENGINE_IMAGE").Value(ctx)
	if err != nil {
		return err
	}
	ref := fmt.Sprintf("%s:%s", engineImage, version)

	digest, err := c.Container().Publish(ctx, ref, dagger.ContainerPublishOpts{
		PlatformVariants: util.DevEngineContainer(c, publishedEngineArches),
	})
	if err != nil {
		return err
	}

	if semver.IsValid(version) {
		sdks := sdk.All{}
		if err := sdks.Bump(ctx, version); err != nil {
			return err
		}
	} else {
		fmt.Printf("'%s' is not a semver version, skipping image bump in SDKs", version)
	}

	time.Sleep(3 * time.Second) // allow buildkit logs to flush, to minimize potential confusion with interleaving
	fmt.Println("PUBLISHED IMAGE REF:", digest)

	return nil
}

// Verify that all arches for the engine can be built. Just do a local export to avoid setting up
// a registry
func (t Engine) TestPublish(ctx context.Context) error {
	c, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return err
	}
	defer c.Close()

	c = c.Pipeline("engine").Pipeline("test-publish")
	_, err = c.Container().Export(ctx, "./engine.tar.gz", dagger.ContainerExportOpts{
		PlatformVariants: util.DevEngineContainer(c, publishedEngineArches),
	})
	return err
}

func (t Engine) test(ctx context.Context, race bool) error {
	c, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return err
	}
	defer c.Close()

	registry := c.Pipeline("registry").Container().From("registry:2").
		WithExposedPort(5000, dagger.ContainerWithExposedPortOpts{Protocol: dagger.Tcp}).
		WithExec(nil)

	const htpasswd = "john:$2y$05$/iP8ud0Fs8o3NLlElyfVVOp6LesJl3oRLYoc3neArZKWX10OhynSC" //nolint:gosec
	privateRegistry := c.Pipeline("private registry").Container().From("registry:2").
		WithNewFile("/auth/htpasswd", dagger.ContainerWithNewFileOpts{Contents: htpasswd}).
		WithEnvVariable("REGISTRY_AUTH", "htpasswd").
		WithEnvVariable("REGISTRY_AUTH_HTPASSWD_REALM", "Registry Realm").
		WithEnvVariable("REGISTRY_AUTH_HTPASSWD_PATH", "/auth/htpasswd").
		WithExposedPort(5000, dagger.ContainerWithExposedPortOpts{Protocol: dagger.Tcp}).
		WithExec(nil)

	opts := util.DevEngineOpts{
		EntrypointArgs: map[string]string{
			"network-name": "dagger-dev",
			"network-cidr": "10.88.0.0/16",
		},
		ConfigEntries: map[string]string{
			"grpc":                            `address=["unix:///var/run/buildkit/buildkitd.sock", "tcp://0.0.0.0:1234"]`,
			`registry."registry:5000"`:        "http = true",
			`registry."privateregistry:5000"`: "http = true",
			`registry."docker.io"`:            `mirrors = ["mirror.gcr.io"]`,
		},
	}
	devEngine := util.DevEngineContainer(c.Pipeline("dev-engine"), []string{runtime.GOARCH}, opts)[0]
	devEngine = devEngine.
		WithServiceBinding("registry", registry).
		WithServiceBinding("privateregistry", privateRegistry).
		WithExposedPort(1234, dagger.ContainerWithExposedPortOpts{Protocol: dagger.Tcp}).
		// TODO: in some ways it's nice to have cache here, in others it may actually result in our tests being less reproducible.
		// Can consider rm -rfing this dir every engine start if we decide we want a clean slate every time.
		// It's important it's a cache mount though because otherwise overlay won't be available
		WithMountedCache("/var/lib/dagger", c.CacheVolume("dagger-dev-engine-state")).
		WithExec(nil, dagger.ContainerWithExecOpts{
			InsecureRootCapabilities:      true,
			ExperimentalPrivilegedNesting: true,
		})

	endpoint, err := devEngine.Endpoint(ctx, dagger.ContainerEndpointOpts{Port: 1234, Scheme: "tcp"})
	if err != nil {
		return err
	}

	c = c.Pipeline("engine").Pipeline("test")

	cgoEnabledEnv := "0"
	args := []string{"go", "test", "-p", "16", "-v", "-count=1", "-timeout=15m"}
	if race {
		args = append(args, "-race", "-timeout=1h")
		cgoEnabledEnv = "1"
	}
	args = append(args, "./...")
	cliBinPath := "/.dagger-cli"

	output, err := util.GoBase(c).
		WithMountedDirectory("/app", util.Repository(c)). // need all the source for extension tests
		WithWorkdir("/app").
		WithServiceBinding("dagger-engine", devEngine).
		WithEnvVariable("CGO_ENABLED", cgoEnabledEnv).
		WithEnvVariable("_EXPERIMENTAL_DAGGER_CACHE_CONFIG", os.Getenv("_EXPERIMENTAL_DAGGER_CACHE_CONFIG")).
		WithMountedFile(cliBinPath, util.DaggerBinary(c)).
		WithEnvVariable("_EXPERIMENTAL_DAGGER_CLI_BIN", cliBinPath).
		WithEnvVariable("_EXPERIMENTAL_DAGGER_RUNNER_HOST", endpoint).
		WithMountedDirectory("/root/.docker", util.HostDockerDir(c)).
		WithExec(args).
		Stdout(ctx)
	if err != nil {
		return err
	}
	fmt.Println(output)
	return nil
}

// Test runs Engine tests
func (t Engine) Test(ctx context.Context) error {
	return t.test(ctx, false)
}

// TestRace runs Engine tests with go race detector enabled
func (t Engine) TestRace(ctx context.Context) error {
	return t.test(ctx, true)
}

func (t Engine) Dev(ctx context.Context) error {
	c, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return err
	}
	defer c.Close()

	c = c.Pipeline("engine").Pipeline("dev")

	tmpfile, err := os.CreateTemp("", "dagger-engine-export")
	if err != nil {
		return err
	}
	defer os.Remove(tmpfile.Name())

	arches := []string{runtime.GOARCH}

	_, err = c.Container().Export(ctx, tmpfile.Name(), dagger.ContainerExportOpts{
		PlatformVariants: util.DevEngineContainer(c, arches),
	})
	if err != nil {
		return err
	}

	volumeName := util.EngineContainerName
	imageName := fmt.Sprintf("localhost/%s:latest", util.EngineContainerName)

	// #nosec
	loadCmd := exec.CommandContext(ctx, "docker", "load", "-i", tmpfile.Name())
	output, err := loadCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker load failed: %w: %s", err, output)
	}
	_, imageID, ok := strings.Cut(string(output), "sha256:")
	if !ok {
		return fmt.Errorf("unexpected output from docker load: %s", output)
	}
	imageID = strings.TrimSpace(imageID)

	if output, err := exec.CommandContext(ctx, "docker",
		"tag",
		imageID,
		imageName,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("docker tag: %w: %s", err, output)
	}

	if output, err := exec.CommandContext(ctx, "docker",
		"rm",
		"-fv",
		util.EngineContainerName,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("docker rm: %w: %s", err, output)
	}

	runArgs := []string{
		"run",
		"-d",
		// "--rm",
		"-e", cacheConfigEnvName,
		"-e", servicesDNSEnvName,
		"-v", volumeName + ":" + engineDefaultStateDir,
		"--name", util.EngineContainerName,
		"--privileged",
	}
	runArgs = append(runArgs, imageName, "--debug")

	if output, err := exec.CommandContext(ctx, "docker", runArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("docker run: %w: %s", err, output)
	}

	// build the CLI and export locally so it can be used to connect to the engine
	binDest := filepath.Join(os.Getenv("DAGGER_SRC_ROOT"), "bin", "dagger")
	_, err = util.HostDaggerBinary(c).Export(ctx, binDest)
	if err != nil {
		return err
	}

	fmt.Println("export _EXPERIMENTAL_DAGGER_CLI_BIN=" + binDest)
	fmt.Println("export _EXPERIMENTAL_DAGGER_RUNNER_HOST=docker-container://" + util.EngineContainerName)
	return nil
}

const cniVersion = "v1.2.0"

func dnsnameBinary(c *dagger.Client, arch string) *dagger.File {
	return util.GoBase(c).
		WithEnvVariable("GOOS", "linux").
		WithEnvVariable("GOARCH", arch).
		WithExec([]string{
			"go", "build",
			"-o", "./bin/dnsname",
			"-ldflags", "-s -w",
			"/app/cmd/dnsname",
		}).
		File("./bin/dnsname")
}

// DevEngineOpts are options for the dev engine
// type DevEngineOpts struct {
// 	EntrypointArgs map[string]string
// 	ConfigEntries  map[string]string
// }

// func getEntrypoint(opts ...DevEngineOpts) (string, error) {
// 	mergedOpts := map[string]string{}
// 	for _, opt := range opts {
// 		maps.Copy(mergedOpts, opt.EntrypointArgs)
// 	}
// 	keys := maps.Keys(mergedOpts)
// 	sort.Strings(keys)

// 	var entrypoint string

// 	type entrypointTmplParams struct {
// 		Bridge            string
// 		EngineBin         string
// 		EngineConfig      string
// 		EntrypointArgs    map[string]string
// 		EntrypointArgKeys []string
// 	}
// 	tmpl := template.Must(template.New("entrypoint").Parse(engineEntrypointTmpl))
// 	buf := new(bytes.Buffer)
// 	err := tmpl.Execute(buf, entrypointTmplParams{
// 		EngineBin:         "/usr/local/bin/" + engineBinName,
// 		EngineConfig:      engineTomlPath,
// 		EntrypointArgs:    mergedOpts,
// 		EntrypointArgKeys: keys,
// 	})
// 	if err != nil {
// 		panic(err)
// 	}
// 	entrypoint = buf.String()

// 	return entrypoint, nil
// }

// func getConfig(opts ...DevEngineOpts) (string, error) {
// 	mergedOpts := map[string]string{}
// 	for _, opt := range opts {
// 		maps.Copy(mergedOpts, opt.ConfigEntries)
// 	}
// 	keys := maps.Keys(mergedOpts)
// 	sort.Strings(keys)

// 	var config string

// 	type configTmplParams struct {
// 		ConfigEntries map[string]string
// 		ConfigKeys    []string
// 	}
// 	tmpl := template.Must(template.New("config").Parse(engineConfigTmpl))
// 	buf := new(bytes.Buffer)
// 	err := tmpl.Execute(buf, configTmplParams{
// 		ConfigEntries: mergedOpts,
// 		ConfigKeys:    keys,
// 	})
// 	if err != nil {
// 		panic(err)
// 	}
// 	config = buf.String()

// 	return config, nil
// }

// func DevEngineContainer(c *dagger.Client, arches []string, opts ...DevEngineOpts) []*dagger.Container {
// 	return devEngineContainer(c, arches, opts...)
// }

// func devEngineContainer(c *dagger.Client, arches []string, opts ...DevEngineOpts) []*dagger.Container {
// 	engineConfig, err := getConfig(opts...)
// 	if err != nil {
// 		panic(err)
// 	}
// 	engineEntrypoint, err := getEntrypoint(opts...)
// 	if err != nil {
// 		panic(err)
// 	}

// 	platformVariants := make([]*dagger.Container, 0, len(arches))
// 	for _, arch := range arches {
// 		platformVariants = append(platformVariants, c.
// 			Container(dagger.ContainerOpts{Platform: dagger.Platform("linux/" + arch)}).
// 			From("alpine:"+alpineVersion).
// 			WithExec([]string{
// 				"apk", "add",
// 				// for Buildkit
// 				"git", "openssh", "pigz", "xz",
// 				// for CNI
// 				"iptables", "ip6tables", "dnsmasq",
// 			}).
// 			WithFile("/usr/local/bin/runc", runcBin(c, arch), dagger.ContainerWithFileOpts{
// 				Permissions: 0700,
// 			}).
// 			WithFile("/usr/local/bin/buildctl", buildctlBin(c, arch)).
// 			WithFile("/usr/local/bin/"+shimBinName, shimBin(c, arch)).
// 			WithFile("/usr/local/bin/"+engineBinName, engineBin(c, arch)).
// 			WithDirectory("/usr/local/bin", qemuBins(c, arch)).
// 			WithDirectory("/opt/cni/bin", cniPlugins(c, arch)).
// 			WithDirectory(engineDefaultStateDir, c.Directory()).
// 			WithNewFile(engineTomlPath, dagger.ContainerWithNewFileOpts{
// 				Contents:    engineConfig,
// 				Permissions: 0600,
// 			}).
// 			WithNewFile(engineEntrypointPath, dagger.ContainerWithNewFileOpts{
// 				Contents:    engineEntrypoint,
// 				Permissions: 755,
// 			}).
// 			WithEntrypoint([]string{"dagger-entrypoint.sh"}),
// 		)
// 	}

// 	return platformVariants
// }

func cniPlugins(c *dagger.Client, arch string) *dagger.Directory {
	cniURL := fmt.Sprintf(
		"https://github.com/containernetworking/plugins/releases/download/%s/cni-plugins-%s-%s-%s.tgz",
		cniVersion, "linux", arch, cniVersion,
	)

	return c.Container().
		From("alpine:"+alpineVersion).
		WithMountedFile("/tmp/cni-plugins.tgz", c.HTTP(cniURL)).
		WithDirectory("/opt/cni/bin", c.Directory()).
		WithExec([]string{
			"tar", "-xzf", "/tmp/cni-plugins.tgz",
			"-C", "/opt/cni/bin",
			// only unpack plugins we actually need
			"./bridge", "./firewall", // required by dagger network stack
			"./loopback", "./host-local", // implicitly required (container fails without them)
		}).
		WithFile("/opt/cni/bin/dnsname", dnsnameBinary(c, arch)).
		Directory("/opt/cni/bin")
}

func engineBin(c *dagger.Client, arch string) *dagger.File {
	return util.GoBase(c).
		WithEnvVariable("GOOS", "linux").
		WithEnvVariable("GOARCH", arch).
		WithExec([]string{
			"go", "build",
			"-o", "./bin/" + engineBinName,
			"-ldflags", "-s -w",
			"/app/cmd/engine",
		}).
		File("./bin/" + engineBinName)
}

func shimBin(c *dagger.Client, arch string) *dagger.File {
	return util.GoBase(c).
		WithEnvVariable("GOOS", "linux").
		WithEnvVariable("GOARCH", arch).
		WithExec([]string{
			"go", "build",
			"-o", "./bin/" + shimBinName,
			"-ldflags", "-s -w",
			"/app/cmd/shim",
		}).
		File("./bin/" + shimBinName)
}

func buildctlBin(c *dagger.Client, arch string) *dagger.File {
	return util.GoBase(c).
		WithEnvVariable("GOOS", "linux").
		WithEnvVariable("GOARCH", arch).
		WithExec([]string{
			"go", "build",
			"-o", "./bin/buildctl",
			"-ldflags", "-s -w",
			"github.com/moby/buildkit/cmd/buildctl",
		}).
		File("./bin/buildctl")
}

func runcBin(c *dagger.Client, arch string) *dagger.File {
	return c.HTTP(fmt.Sprintf(
		"https://github.com/opencontainers/runc/releases/download/%s/runc.%s",
		runcVersion,
		arch,
	))
}

func qemuBins(c *dagger.Client, arch string) *dagger.Directory {
	return c.
		Container(dagger.ContainerOpts{Platform: dagger.Platform("linux/" + arch)}).
		From(qemuBinImage).
		Rootfs()
}
