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
package build

import (
	"context"
	"fmt"
	buildkit "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progresswriter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/opts"
	"golang.org/x/xerrors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
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

	buildCtx string
	solveOpt buildkit.SolveOpt

	buildkitAddrs []string
}

func newBuilderOptions(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *BuildOptions {
	return &BuildOptions{
		GlobalOptions: opts,
		buildCtx:      ".",
		solveOpt: buildkit.SolveOpt{
			Frontend: "dockerfile.v0",
		},
	}
}

func (o *BuildOptions) Complete(cmd *cobra.Command, args []string) error {
	clientset, err := o.ClientSet()
	if err != nil {
		return err
	}

	if len(o.buildkitAddrs) == 0 {
		o.buildkitAddrs, err = fetchBuilderEndpoints(clientset)
		if err != nil {
			return err
		}
	}

	buildCtx := "."
	if len(args) > 0 {
		o.buildCtx = args[0]
	}

	dockerfile := o.dockerfile
	if len(dockerfile) == 0 {
		dockerfile = filepath.Join(o.buildCtx, "Dockerfile")
	}

	o.solveOpt.LocalDirs = map[string]string{
		"context":    buildCtx,
		"dockerfile": filepath.Dir(dockerfile),
	}

	o.solveOpt.FrontendAttrs = map[string]string{
		"filename": filepath.Base(dockerfile),
	}

	if len(o.targetStage) > 0 {
		o.solveOpt.FrontendAttrs["target"] = o.targetStage
	}

	if o.noCache {
		o.solveOpt.FrontendAttrs["no-cache"] = ""
	}

	for _, buildArg := range o.buildArgs {
		kv := strings.SplitN(buildArg, "=", 2)
		if len(kv) != 2 {
			return errors.Errorf("invalid --build-arg value %s", buildArg)
		}

		o.solveOpt.FrontendAttrs["build-arg:"+kv[0]] = kv[1]
	}

	if len(o.tag) == 0 && len(o.localDir) == 0 {
		return fmt.Errorf("set either a tag or a local path")
	}

	if len(o.tag) > 0 {
		o.solveOpt.Exports = append(o.solveOpt.Exports, buildkit.ExportEntry{
			Type: "image",
			Attrs: map[string]string{
				"name": o.tag,
			},
		})
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

	return nil
}

func (o *BuildOptions) Validate() error {
	return nil
}

func (o *BuildOptions) Run() (err error) {
	var client *buildkit.Client
	for i, addr := range o.buildkitAddrs {
		client, err = buildkit.New(context.TODO(), addr, buildkit.WithFailFast())
		if err == nil {
			timed, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
			_, err = client.ListWorkers(timed)
			cancel()
		}

		if err == nil {
			break
		}

		fmt.Fprintf(os.Stderr, `can't connect to builder "%s": %s\n`, addr, err)
		i++
		if i < len(o.buildkitAddrs) {
			fmt.Fprintf(os.Stderr, `Try the next endpoint %s\n`, o.buildkitAddrs[i])
		}
	}

	if client == nil {
		return xerrors.Errorf("all builder endpoints are unavailable")
	}

	defer client.Close()

	pw, err := progresswriter.NewPrinter(context.TODO(), os.Stderr, "")
	if err != nil {
		return xerrors.Errorf("can't initialize progress writer: %s", err)
	}

	if _, err = client.Solve(context.TODO(), nil, o.solveOpt, pw.Status()); err != nil {
		<-pw.Done()
		return xerrors.Errorf("%s", err)
	}

	<-pw.Done()

	return nil
}

func NewCmd(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *cobra.Command {
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
		SilenceErrors: false,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(cmd, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
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

	o.AddFlags(cmd.Flags())

	cmd.AddCommand(newCmdInstall(opts, streams))
	return cmd
}
