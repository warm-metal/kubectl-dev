package app

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	appcorev1 "github.com/warm-metal/cliapp/pkg/apis/cliapp/v1"
	appv1 "github.com/warm-metal/cliapp/pkg/clientset/versioned"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/opts"
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type appInstallOptions struct {
	*opts.GlobalOptions
	genericclioptions.IOStreams
	shortcutInstallOptions

	name      string
	namespace string

	image      string
	dockerfile string

	hostPaths []string
	envs      []string

	distro string
	shell  string

	useHttpProxy bool

	app *appcorev1.CliApp
}

func (o *appInstallOptions) Complete(_ *cobra.Command, args []string) error {
	if o.Raw().Namespace != nil && len(*o.Raw().Namespace) > 0 {
		o.namespace = *o.Raw().Namespace
	}

	if err := o.shortcutInstallOptions.init(o.name); err != nil {
		return err
	}

	o.app = &appcorev1.CliApp{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.name,
			Namespace: o.namespace,
		},
		Spec: appcorev1.CliAppSpec{
			Image:       o.image,
			Dockerfile:  o.dockerfile,
			Command:     args,
			HostPath:    o.hostPaths,
			Env:         o.envs,
			TargetPhase: appcorev1.CliAppPhaseRest,
		},
	}

	if len(args) == 0 {
		o.app.Spec.Command = []string{o.name}
	}

	if len(o.distro) > 0 {
		distro, err := utils.ValidateDistro(o.distro)
		if err != nil {
			return err
		}

		o.app.Spec.Distro = distro
	}

	if len(o.shell) > 0 {
		shell, err := utils.ValidateShell(o.shell)
		if err != nil {
			return err
		}

		o.app.Spec.Shell = shell
	}

	if o.useHttpProxy {
		proxies, err := utils.GetSysProxyEnvs()
		if err != nil {
			return err
		}
		o.app.Spec.Env = append(o.app.Spec.Env, proxies...)
	}

	return nil
}

func (o *appInstallOptions) Validate() error {
	return nil
}

func (o *appInstallOptions) Run(ctx context.Context) error {
	conf, err := o.Raw().ToRESTConfig()
	if err != nil {
		return err
	}

	appClient, err := appv1.NewForConfig(conf)
	if err != nil {
		return err
	}

	app, err := appClient.CliappV1().CliApps(o.app.Namespace).Get(ctx, o.app.Name, metav1.GetOptions{})
	if err == nil {
		app.Spec = o.app.Spec
		_, err = appClient.CliappV1().CliApps(o.app.Namespace).Update(ctx, app, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err = appClient.CliappV1().CliApps(o.app.Namespace).Create(ctx, o.app, metav1.CreateOptions{})
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				return err
			}

		}
	}

	if err := o.installShortcut(o.app.Name, o.app.Namespace); err != nil {
		return err
	}

	fmt.Println("Installed")
	return nil
}

func newAppInstallCmd(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *cobra.Command {
	o := &appInstallOptions{
		GlobalOptions:          opts,
		IOStreams:              streams,
		namespace:              metav1.NamespaceDefault,
		shortcutInstallOptions: initShortcutInstallOptions(),
	}

	var cmd = &cobra.Command{
		Use:   "install [OPTIONS] [command]",
		Short: "Install an CliApp.",
		Long:  `A CliApp is executed in any local terminal then runs in a K8s cluster.`,
		Example: `# Install ctr via an image to work with node containerd, then, you can run "ctr i ls" to show all images.
# The last argument "crictl" shows that command crictl will be executed in the Pod once the app is executed.
# If omitted, the command same with the app name is started instead.
sudo -E kubectl dev app install --name ctr -n default --env CONTAINERD_NAMESPACE=k8s.io \
	--hostpath /var/run/containerd/containerd.sock \
	--image docker.io/warmmetal/ctr:v1 \
	--use-proxy \
	ctr

# Install ctr via a Dockerfile.
sudo -E kubectl dev app install --name ctr -n default --env CONTAINERD_NAMESPACE=k8s.io \
	--hostpath /var/run/containerd/containerd.sock \
	--dockerfile https://raw.githubusercontent.com/warm-metal/cliapps/master/ctr/Dockerfile \
	--use-proxy
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

	cmd.Flags().StringVar(&o.name, "name", "", "App name")
	cmd.Flags().StringVar(&o.image, "image", "", "Image the app uses")
	cmd.Flags().StringVar(&o.dockerfile, "dockerfile", "", "Dockerfile to build image that the app uses")
	cmd.Flags().StringSliceVar(&o.hostPaths, "hostpath", nil, "Host paths to be mounted")
	cmd.Flags().StringSliceVar(&o.envs, "env", nil, "Environment variables")
	cmd.Flags().BoolVar(&o.useHttpProxy, "use-proxy", false, "If set, use current HTTP proxy settings.")
	cmd.Flags().StringVar(&o.distro, "distro", "",
		"Linux distro that the app prefer. The default value is alpine.")
	cmd.Flags().StringVar(&o.shell, "shell", "",
		"The shell you prefer. The default value is bash. You can also use zsh instead.")
	cmd.Flags().StringVar(&o.shortcutRoot, "install-base", o.shortcutRoot,
		"Directory where app to be installed. It should be one of the PATH.")
	o.AddPersistentFlags(cmd.Flags())

	return cmd
}
