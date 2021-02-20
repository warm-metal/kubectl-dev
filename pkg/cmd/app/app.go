package app

import (
	"context"
	"github.com/spf13/cobra"
	"github.com/warm-metal/cliapp-session-gate/pkg/libcli"
	appv1 "github.com/warm-metal/cliapp/pkg/clientset/versioned"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/opts"
	"golang.org/x/xerrors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type AppOptions struct {
	*opts.GlobalOptions
	genericclioptions.IOStreams

	name      string
	namespace string

	args []string
}

func (o *AppOptions) Complete(cmd *cobra.Command, args []string) error {
	if o.Raw().Namespace != nil && len(*o.Raw().Namespace) > 0 {
		o.namespace = *o.Raw().Namespace
	}

	o.args = args
	return nil
}

func (o *AppOptions) Validate() error {
	return nil
}

func (o *AppOptions) Run() error {
	config, err := o.Raw().ToRESTConfig()
	if err != nil {
		return err
	}

	appClient, err := appv1.NewForConfig(config)
	if err != nil {
		return err
	}

	app, err := appClient.CliappV1().CliApps(o.namespace).Get(context.TODO(), o.name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	clientset, err := o.ClientSet()
	if err != nil {
		return err
	}

	endpoints, err := libcli.FetchGateEndpoints(clientset)
	if err != nil {
		return err
	}

	err = libcli.ExecCliApp(endpoints, app, o.args, o.In, o.Out)
	if err != nil {
		return xerrors.Errorf("can't open app shell: %s", err)
	}

	return nil
}

func NewCmd(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *cobra.Command {
	o := &AppOptions{
		GlobalOptions: opts,
		IOStreams:     streams,
	}

	var cmd = &cobra.Command{
		Use:   "app [OPTIONS] command",
		Short: "Run a CliApp.",
		Long:  `Run a installed CliApp`,
		Example: `# Run an app

`,
		SilenceUsage:  true,
		SilenceErrors: true,
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

	cmd.Flags().StringVar(&o.name, "name", "", "App name. A random name would be used if not set.")
	o.AddFlags(cmd.Flags())

	cmd.AddCommand(
		newAppInstallCmd(opts, streams),
		newAppUninstallCmd(opts, streams),
		newAppListCmd(opts, streams),
	)
	return cmd
}
