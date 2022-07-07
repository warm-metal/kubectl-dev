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
	buildkit "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/progress/progresswriter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/opts"
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BuildOptions struct {
	*opts.GlobalOptions

	dockerfile  string
	tag         string
	localDir    string
	targetStage string
	noCache     bool
	buildArgs   []string
	push        bool
	insecure    bool
	platform    string

	solveOpt buildkit.SolveOpt

	buildkitAddrs []string
}

func newBuilderOptions(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *BuildOptions {
	return &BuildOptions{
		GlobalOptions: opts,
		solveOpt: buildkit.SolveOpt{
			Frontend:      "dockerfile.v0",
			FrontendAttrs: make(map[string]string),
		},
	}
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

	for _, buildArg := range o.buildArgs {
		kv := strings.SplitN(buildArg, "=", 2)
		if len(kv) != 2 {
			return errors.Errorf("invalid --build-arg value %s", buildArg)
		}

		o.solveOpt.FrontendAttrs["build-arg:"+kv[0]] = kv[1]
	}

	buildCtx := "."
	if len(args) > 0 {
		buildCtx = args[0]
	}

	if len(o.tag) == 0 && len(o.localDir) == 0 {
		absCtx, err := filepath.Abs(buildCtx)
		if err != nil {
			return err
		}
		o.tag = fmt.Sprintf("build.local/x/%s:v0.0.0", filepath.Base(absCtx))
		fmt.Printf("Neither image tag nor local binary is given. \nAssuming to build a local image for testing: %s\n", o.tag)
	}

	if len(o.tag) > 0 {
		export := buildkit.ExportEntry{
			Type: "image",
			Attrs: map[string]string{
				"name": o.tag,
			},
		}

		if o.push {
			export.Attrs["push"] = "true"
		}

		// FIXME test if the target registry is insecure.
		if o.insecure {
			export.Attrs["registry.insecure"] = "true"
		}

		o.solveOpt.Exports = append(o.solveOpt.Exports, export)
	}

	if len(o.localDir) > 0 {
		o.solveOpt.Exports = append(o.solveOpt.Exports, buildkit.ExportEntry{
			Type: "local",
			Attrs: map[string]string{
				"dest": o.localDir,
			},
			OutputDir: o.localDir,
		})
	}

	if u, err := url.Parse(o.dockerfile); err == nil && strings.HasPrefix(u.Scheme, "http") {
		o.solveOpt.FrontendAttrs["context"] = o.dockerfile
		return nil
	}

	dockerfile := o.dockerfile
	if len(dockerfile) == 0 {
		dockerfile = filepath.Join(buildCtx, "Dockerfile")
	} else if !filepath.IsAbs(dockerfile) {
		dockerfile = filepath.Join(buildCtx, dockerfile)
	}

	o.solveOpt.LocalDirs = map[string]string{
		"context":    buildCtx,
		"dockerfile": filepath.Dir(dockerfile),
	}

	o.solveOpt.FrontendAttrs["filename"] = filepath.Base(dockerfile)

	if len(o.targetStage) > 0 {
		o.solveOpt.FrontendAttrs["target"] = o.targetStage
	}

	if o.noCache {
		o.solveOpt.FrontendAttrs["no-cache"] = ""
	}

	if len(o.platform) > 0 {
		o.solveOpt.FrontendAttrs["platform"] = o.platform
	}

	o.solveOpt.Session = []session.Attachable{authprovider.NewDockerAuthProvider(os.Stderr)}
	return nil
}

func (o *BuildOptions) Validate() error {
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

	if _, err = client.Solve(ctx, nil, o.solveOpt, pw.Status()); err != nil {
		<-pw.Done()
		return fmt.Errorf("%s", err)
	}

	<-pw.Done()

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

	cmd.Flags().StringVarP(&o.dockerfile, "file", "f", "",
		"Name of the Dockerfile (Default is 'PATH/Dockerfile')")
	cmd.Flags().StringVarP(&o.tag, "tag", "t", "",
		"Name and optionally a tag in the 'name:tag' format")
	cmd.Flags().StringVar(&o.localDir, "local", "",
		"Build binaries instead an image and copy them to the specified path.")
	cmd.Flags().StringVar(&o.targetStage, "target", "", "Set the target build stage to build.")
	cmd.Flags().BoolVar(&o.noCache, "no-cache", false, "Do not use cache when building.")
	cmd.Flags().StringSliceVar(&o.buildArgs, "build-arg", nil, "Set build-time variables.")
	cmd.Flags().StringSliceVar(&o.buildkitAddrs, "buildkit-addr", nil,
		"Endpoints of the buildkitd. Must be a valid tcp or unix socket URL(tcp:// or unix://). If not set, "+
			"automatically fetch them from the cluster")
	cmd.Flags().BoolVar(&o.push, "push", false, "Push the image.")
	cmd.Flags().BoolVar(&o.insecure, "insecure", false, "Enable if the target registry is insecure.")
	cmd.Flags().StringVar(&o.platform, "platform", "", "Set target platform for build.")

	o.AddPersistentFlags(cmd.Flags())
	return cmd
}
