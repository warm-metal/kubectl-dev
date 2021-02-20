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
	"encoding/base64"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/warm-metal/cliapp-session-gate/pkg/libcli"
	appcorev1 "github.com/warm-metal/cliapp/pkg/apis/cliapp/v1"
	appv1 "github.com/warm-metal/cliapp/pkg/clientset/versioned"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/opts"
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"golang.org/x/xerrors"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"strings"
)

type DebugOptions struct {
	*opts.GlobalOptions
	genericclioptions.IOStreams

	container    string
	useHTTPProxy bool

	instance  string
	image     string
	namespace string

	kindAndName string
	distro      string
	shell       string
}

func NewDebugOptions(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *DebugOptions {
	return &DebugOptions{
		GlobalOptions: opts,
		IOStreams:     streams,
		namespace:     metav1.NamespaceDefault,
	}
}

func (o *DebugOptions) Complete(cmd *cobra.Command, args []string) error {
	if o.Raw().Namespace != nil && len(*o.Raw().Namespace) > 0 {
		o.namespace = *o.Raw().Namespace
	}

	if len(args) == 0 {
		if len(o.image) == 0 {
			return xerrors.Errorf("specify an image or an object")
		}

		encodedImage := base64.StdEncoding.EncodeToString([]byte(o.image))
		o.instance = fmt.Sprintf("debugger-%s", encodedImage)
		return nil
	}

	o.kindAndName = strings.Join(args, "/")
	o.instance = fmt.Sprintf("debugger-%s", strings.Replace(o.kindAndName, "/", "-", -1))
	return nil
}

func (o *DebugOptions) Validate() error {
	if len(o.image) == 0 && len(o.kindAndName) == 0 {
		return fmt.Errorf("an image or object is required. See the usage")
	}

	return nil
}

func (o *DebugOptions) Run() error {
	conf, err := o.Raw().ToRESTConfig()
	if err != nil {
		return err
	}

	appClient, err := appv1.NewForConfig(conf)
	if err != nil {
		return err
	}

	app, err := appClient.CliappV1().CliApps(o.namespace).Get(context.TODO(), o.instance, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		app = &appcorev1.CliApp{
			ObjectMeta: metav1.ObjectMeta{
				Name:      o.instance,
				Namespace: o.namespace,
			},
			Spec: appcorev1.CliAppSpec{
				TargetPhase:         appcorev1.CliAppPhaseLive,
				UninstallUnlessLive: true,
				Distro:              appcorev1.CliAppDistroAlpine,
				Shell:               appcorev1.CliAppShellBash,
			},
		}

		if len(o.kindAndName) > 0 {
			app.Spec.ForkObject = o.kindAndName
			app.Spec.ForkContainer = o.container
		} else {
			app.Spec.Image = o.image
		}

		if o.useHTTPProxy {
			proxies, err := utils.GetSysProxyEnvs()
			if err != nil {
				return err
			}

			if len(proxies) == 0 {
				fmt.Fprintln(o.ErrOut, "http proxy doesn't set.")
			}

			app.Spec.Env = append(app.Spec.Env, proxies...)
		}

		if len(o.distro) > 0 {
			distro, err := utils.ValidateDistro(o.distro)
			if err != nil {
				return err
			}

			app.Spec.Distro = distro
		}

		if len(o.shell) > 0 {
			shell, err := utils.ValidateShell(o.shell)
			if err != nil {
				return err
			}

			app.Spec.Shell = shell
		}

		app, err = appClient.CliappV1().CliApps(app.Namespace).Create(context.TODO(), app, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}

	clientset, err := o.ClientSet()
	if err != nil {
		return err
	}

	endpoints, err := libcli.FetchGateEndpoints(clientset)
	if err != nil {
		return err
	}

	err = libcli.ExecCliApp(endpoints, app, []string{string(app.Spec.Shell)}, o.In, o.Out)
	if err != nil {
		return xerrors.Errorf("can't open app shell: %s", err)
	}

	return nil
}

func NewCmdDebug(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDebugOptions(opts, streams)

	var cmd = &cobra.Command{
		Use:   "debug",
		Short: "Debug running or failed workloads or images.",
		Long: `The image of the target workload will be mounted to a new Pod. You will see all original configurations 
even the filesystem in the new Pod, except the same entrypoint. Workloads could be Deployment, StatefulSet, DaemonSet,
ReplicaSet, Job, CronJob, and Pod.

The command requires the CSI driver https://github.com/warm-metal/csi-driver-image. All the install manifests are in its
"install" folder. If they aren't exactly match your cluster, you can install it manually. 
`,
		Example: `# # Debug a running or failed workload. Run the same command again could open a new session to the same debugger.
kubectl dev debug deploy foo

# Specify container name if more than one containers in the Pod.
kubectl dev debug ds foo -c bar

# Debug a Pod with a new versioned image. 
kubectl dev debug pod foo --image bar:new-version

#Debug an image.
kubectl dev debug --image foo:latest

# Use local network proxies.
kubectl dev debug cronjob foo --use-proxy
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

	cmd.Flags().StringVarP(
		&o.container, "container", "c", o.container,
		"Container of the specified object if in which there are multiple containers")
	cmd.Flags().StringVar(&o.image, "image", "",
		"The target image. If not set, use the image which the object used.")
	cmd.Flags().BoolVar(&o.useHTTPProxy, "use-proxy", false,
		"If set, use current HTTP proxy settings.")
	cmd.Flags().StringVar(&o.distro, "distro", "",
		"Linux distro that the app prefer. The default value is alpine.")
	cmd.Flags().StringVar(&o.shell, "shell", "",
		"The shell you prefer. The default value is bash. You can also use zsh instead.")
	o.AddFlags(cmd.Flags())

	return cmd
}
