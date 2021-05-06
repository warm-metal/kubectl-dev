package app

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	appv1 "github.com/warm-metal/cliapp/pkg/clientset/versioned"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/opts"
	"golang.org/x/xerrors"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type appUninstallOptions struct {
	*opts.GlobalOptions
	genericclioptions.IOStreams
	shortcutInstallOptions

	name      string
	namespace string
}

func (o *appUninstallOptions) Complete(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		cmd.SilenceUsage = false
		return xerrors.Errorf("invalid arguments")
	}

	o.name = args[0]

	if o.Raw().Namespace != nil && len(*o.Raw().Namespace) > 0 {
		o.namespace = *o.Raw().Namespace
	}

	if err := o.shortcutInstallOptions.init(o.name); err != nil {
		return err
	}

	return nil
}

func (o *appUninstallOptions) Validate() error {
	return nil
}

func (o *appUninstallOptions) Run(ctx context.Context) error {
	conf, err := o.Raw().ToRESTConfig()
	if err != nil {
		return err
	}

	appClient, err := appv1.NewForConfig(conf)
	if err != nil {
		return err
	}

	err = appClient.CliappV1().CliApps(o.namespace).Delete(ctx, o.name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if err = o.uninstallShortcut(); err != nil {
		return err
	}

	fmt.Println("Uninstalled")
	return err
}

func newAppUninstallCmd(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *cobra.Command {
	o := &appUninstallOptions{
		GlobalOptions:          opts,
		IOStreams:              streams,
		namespace:              metav1.NamespaceDefault,
		shortcutInstallOptions: initShortcutInstallOptions(),
	}

	var cmd = &cobra.Command{
		Use:   "uninstall [OPTIONS] name",
		Short: "Uninstall an CliApp.",
		Long:  `Uninstall an CliApp in the cluster.`,
		Example: `# Uninstall an CliApp
kubectl dev app uninstall -n app ctr
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

	cmd.Flags().StringVar(&o.shortcutRoot, "install-base", o.shortcutRoot,
		"Directory where app to be installed. It should be one of the PATH.")
	o.AddPersistentFlags(cmd.Flags())
	return cmd
}
