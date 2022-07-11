/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"fmt"
	"github.com/warm-metal/kubectl-dev/pkg/conf"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	buildkit "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/progress/progresswriter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/opts"
	"github.com/warm-metal/kubectl-dev/pkg/kubectl"
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"io/ioutil"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	buildConfFile      = "build"
	defaultDockerfile  = ""
	defaultTag         = ""
	defaultLocalDir    = ""
	defaultTargetStage = ""
	defaultPlatform    = ""
)

type BuildContext struct {
	Dockerfile     string `yaml:"dockerfile,omitempty"`
	Tag            string `yaml:"tag,omitempty"`
	AutoTagPattern string `yaml:"auto_tag_pattern,omitempty"`
	LocalDir       string `yaml:"local_dir,omitempty"`
	TargetStage    string `yaml:"target_stage,omitempty"`
	Platform       string `yaml:"platform,omitempty"`

	PathToManifest  string `yaml:"path_to_manifest,omitempty"`
	BuildContextDir string `yaml:"build_context_dir,omitempty"`

	Count int `yaml:"count"`

	solveOpt *buildkit.SolveOpt `yaml:"-"`
}

func (c BuildContext) isDefault() bool {
	return c.Dockerfile == defaultDockerfile && c.Tag == defaultTag &&
		c.LocalDir == defaultLocalDir && c.TargetStage == defaultTargetStage &&
		c.Platform == defaultPlatform
}

// mapping from dockerfile+stage to BuildContext
type DirBuildContext map[string]BuildContext

// mapping from build directory to DirBuildContext
type BuildConfig map[string]DirBuildContext

type BuildOptions struct {
	*opts.GlobalOptions

	BuildContext
	noCache   bool
	buildArgs []string
	push      bool
	insecure  bool
	noProxy   bool

	solveOpt *buildkit.SolveOpt

	buildkitAddrs []string

	config DirBuildContext
}

func newBuilderOptions(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *BuildOptions {
	return &BuildOptions{
		GlobalOptions: opts,
	}
}

func (o *BuildOptions) buildSolveOpt(bc *BuildContext) (*buildkit.SolveOpt, error) {
	solveOpt := buildkit.SolveOpt{
		Frontend:      "dockerfile.v0",
		FrontendAttrs: map[string]string{},
	}

	for _, buildArg := range o.buildArgs {
		kv := strings.SplitN(buildArg, "=", 2)
		if len(kv) != 2 {
			return nil, errors.Errorf("invalid --build-arg value %s", buildArg)
		}

		solveOpt.FrontendAttrs["build-arg:"+kv[0]] = kv[1]
	}

	tag := bc.Tag
	if len(bc.Tag) == 0 && len(bc.LocalDir) == 0 && len(bc.AutoTagPattern) > 0 {
		absCtx, err := filepath.Abs(bc.BuildContextDir)
		if err != nil {
			return nil, err
		}
		tag = fmt.Sprintf(bc.AutoTagPattern, filepath.Base(absCtx), bc.Count)
		fmt.Printf("Neither image tag nor local binary is given. \nAssuming to build a local image for testing: %s\n", bc.Tag)
	}

	if len(tag) > 0 {
		export := buildkit.ExportEntry{
			Type: "image",
			Attrs: map[string]string{
				"name": tag,
			},
		}

		if o.push {
			export.Attrs["push"] = "true"
		}

		if o.insecure {
			export.Attrs["registry.insecure"] = "true"
		}

		solveOpt.Exports = append(solveOpt.Exports, export)
	}

	if len(bc.LocalDir) > 0 {
		solveOpt.Exports = append(solveOpt.Exports, buildkit.ExportEntry{
			Type: "local",
			Attrs: map[string]string{
				"dest": bc.LocalDir,
			},
			OutputDir: bc.LocalDir,
		})
	}

	if u, err := url.Parse(bc.Dockerfile); err == nil && strings.HasPrefix(u.Scheme, "http") {
		solveOpt.FrontendAttrs["context"] = bc.Dockerfile
		return &solveOpt, nil
	}

	dockerfile := bc.Dockerfile
	if len(dockerfile) == 0 {
		dockerfile = filepath.Join(bc.BuildContextDir, "Dockerfile")
	} else if !filepath.IsAbs(dockerfile) {
		dockerfile = filepath.Join(bc.BuildContextDir, dockerfile)
	}

	solveOpt.LocalDirs = map[string]string{
		"context":    bc.BuildContextDir,
		"dockerfile": filepath.Dir(dockerfile),
	}

	solveOpt.FrontendAttrs["filename"] = filepath.Base(dockerfile)

	if len(bc.TargetStage) > 0 {
		solveOpt.FrontendAttrs["target"] = bc.TargetStage
	}

	if o.noCache {
		solveOpt.FrontendAttrs["no-cache"] = ""
	}

	if len(bc.Platform) > 0 {
		solveOpt.FrontendAttrs["platform"] = bc.Platform
	}

	if !o.noProxy {
		const buildArgPrefix = "build-arg:"
		proxies, err := utils.GetSysProxy()
		if err == nil {
			for _, proxy := range proxies {
				solveOpt.FrontendAttrs[buildArgPrefix+proxy.Name] = proxy.Value
			}
		} else {
			fmt.Println(err.Error())
		}
	}

	solveOpt.Session = []session.Attachable{authprovider.NewDockerAuthProvider(os.Stderr)}
	return &solveOpt, nil
}

func (o *BuildOptions) Complete(cmd *cobra.Command, args []string) error {
	clientset, err := o.ClientSet()
	if err != nil {
		return err
	}

	if len(o.buildkitAddrs) == 0 {
		o.buildkitAddrs, err = utils.FetchServiceEndpoints(cmd.Context(), clientset,
			"cliapp-system", "buildkitd", "buildkitd")
		if err != nil {
			return err
		}
	}

	if o.BuildContext.isDefault() {
		config := make(BuildConfig)
		workdir, err := os.Getwd()
		if err != nil {
			return err
		}

		if err = conf.Load(buildConfFile, &config); err == nil {
			o.config = config[workdir]
			for k := range o.config {
				bc := o.config[k]
				bc.solveOpt, err = o.buildSolveOpt(&bc)
				if err != nil {
					return err
				}
				o.config[k] = bc
			}
		} else {
			return errors.New("more arguments are required")
		}
	} else {
		o.BuildContextDir = "."
		if len(args) > 0 {
			o.BuildContextDir = args[0]
		}

		if o.solveOpt, err = o.buildSolveOpt(&o.BuildContext); err != nil {
			return err
		}
	}

	return nil
}

func (o *BuildOptions) Validate() error {
	return nil
}

func solve(
	ctx context.Context, client *buildkit.Client, pw progresswriter.Writer,
	solveOpt *buildkit.SolveOpt, config *BuildContext,
) error {
	_, err := client.Solve(ctx, nil, *solveOpt, pw.Status())
	if err != nil {
		<-pw.Done()
		return fmt.Errorf("%s", err)
	}

	config.Count++

	if len(config.PathToManifest) > 0 {
		manifest, err := ioutil.ReadFile(config.PathToManifest)
		if err == os.ErrNotExist {
			manifest, err = ioutil.ReadFile(filepath.Join(config.BuildContextDir, config.PathToManifest))
		}

		if err == nil {
			fmt.Println("Applying manifests")
			imagePattern := regexp.MustCompile(`(?m)image:.+$`)
			match := imagePattern.FindAll(manifest, -1)
			if len(match) > 1 {
				fmt.Fprintf(os.Stderr, "More than one image found in manifest.\n")
				return nil
			}

			if len(match) == 1 {
				image := ""
				for _, export := range solveOpt.Exports {
					if export.Type == "image" {
						image = export.Attrs["name"]
					}
				}
				manifest = imagePattern.ReplaceAll(manifest, []byte(fmt.Sprintf("image: %s", image)))
			}

			err = kubectl.ApplyManifestsFromStdin(strings.NewReader(string(manifest)))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error applying manifests: %s\n", err)
			}
		}
	}

	return nil
}

func (o *BuildOptions) Run(ctx context.Context) (err error) {
	var client *buildkit.Client
	for i, addr := range o.buildkitAddrs {
		client, err = buildkit.New(ctx, addr, buildkit.WithFailFast())
		if err == nil {
			timed, cancel := context.WithTimeout(ctx, 3*time.Second)
			_, err = client.ListWorkers(timed)
			cancel()
		}

		if err == nil {
			break
		}

		fmt.Fprintf(os.Stderr, `can't connect to builder "%s": %s`+"\n", addr, err)
		i++
		if i < len(o.buildkitAddrs) {
			fmt.Fprintf(os.Stderr, `Try the next endpoint %s`+"\n", o.buildkitAddrs[i])
		}
	}

	if client == nil {
		return fmt.Errorf("all builder endpoints are unavailable")
	}

	defer client.Close()

	pw, err := progresswriter.NewPrinter(ctx, os.Stderr, "")
	if err != nil {
		return fmt.Errorf("can't initialize progress writer: %s", err)
	}

	if o.config != nil {
		for _, config := range o.config {
			if err = solve(ctx, client, pw, config.solveOpt, &config); err != nil {
				return err
			}
		}
	} else {
		if err = solve(ctx, client, pw, o.solveOpt, &o.BuildContext); err != nil {
			return err
		}
	}

	<-pw.Done()

	config := make(BuildConfig)
	workdir, err := os.Getwd()
	if err != nil {
		return err
	}
	if err = conf.Load(buildConfFile, &config); err == nil {
		return err
	}
	config[workdir] = DirBuildContext{
		o.Dockerfile + "/" + o.TargetStage: o.BuildContext,
	}
	if err = conf.Save(buildConfFile, config); err != nil {
		return err
	}

	return nil
}

func NewCmdBuild(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *cobra.Command {
	o := newBuilderOptions(opts, streams)

	var cmd = &cobra.Command{
		Use:   "build [OPTIONS] [PATH]",
		Short: "Build image using Dockerfile",
		Long: `Build images in clusters and share arguments and options with the "docker build" command.
"kubectl-dev build" use buildkitd as its build engine. Since buildkitd only support containerd or oci 
as its worker, the build command also only support containerd as the container runtime.`,
		Example: `# Build image in the cluster using docker parameters and options.
kubectl dev build -t foo:latest -f Dockerfile .

# Build a binary and save to a local directory.
kubectl dev build -f Dockerfile --local foo/bar/ .

# Build image then apply a manifest.
kubectl dev build -t foo:latest -f Dockerfile --manifest foo/bar/manifest.yaml
`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(cmd, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Run(cmd.Context()); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&o.Dockerfile, "file", "f", defaultDockerfile,
		"Name of the Dockerfile (Default is 'PATH/Dockerfile')")
	cmd.Flags().StringVarP(&o.Tag, "tag", "t", defaultTag,
		"Image name and optionally a tag in the 'name:tag' format")
	cmd.Flags().StringVar(&o.AutoTagPattern, "tag-pattern", "build.local/x/%s:v%d",
		"Pattern to generate image name if no tag is given")
	cmd.Flags().StringVar(&o.LocalDir, "local", defaultLocalDir,
		"Build binaries instead an image and copy them to the specified path.")
	cmd.Flags().StringVar(&o.TargetStage, "target", defaultTargetStage, "Set the target build stage to build.")
	cmd.Flags().BoolVar(&o.noCache, "no-cache", false, "Do not use cache when building.")
	cmd.Flags().StringSliceVar(&o.buildArgs, "build-arg", nil, "Set build-time variables.")
	cmd.Flags().StringSliceVar(&o.buildkitAddrs, "buildkit-addr", nil,
		"Endpoints of the buildkitd. Must be a valid tcp or unix socket URL(tcp:// or unix://). If not set, "+
			"automatically fetch them from the cluster")
	cmd.Flags().BoolVar(&o.push, "push", false, "Push the image.")
	cmd.Flags().BoolVar(&o.insecure, "insecure", false, "Enable if the target registry is insecure.")
	cmd.Flags().StringVar(&o.Platform, "platform", defaultPlatform, "Set target platform for build.")
	cmd.Flags().StringVar(&o.PathToManifest, "manifest", "hack/manifests/k8s.yaml",
		"Path to the manifest to be applied after building.")
	cmd.Flags().BoolVar(&o.noProxy, "no-proxy", false, "Do not pass through local proxy configuration when building.")

	o.AddPersistentFlags(cmd.Flags())
	return cmd
}
