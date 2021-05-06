package app

import (
	"context"
	"github.com/spf13/cobra"
	appcorev1 "github.com/warm-metal/cliapp/pkg/apis/cliapp/v1"
	appv1 "github.com/warm-metal/cliapp/pkg/clientset/versioned"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/opts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
)

type appListOptions struct {
	*opts.GlobalOptions
	genericclioptions.IOStreams

	name      string
	namespace string
}

func (o *appListOptions) Complete(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		o.name = args[0]
	}

	return nil
}

func (o *appListOptions) Validate() error {
	return nil
}

func (o *appListOptions) Run(ctx context.Context) error {
	conf, err := o.Raw().ToRESTConfig()
	if err != nil {
		return err
	}

	appClient, err := appv1.NewForConfig(conf)
	if err != nil {
		return err
	}

	filter := metav1.ListOptions{}
	if len(o.name) > 0 {
		filter.FieldSelector = fields.Set{"metadata.name": o.name}.AsSelector().String()
	}

	apps, err := appClient.CliappV1().CliApps(o.namespace).List(ctx, filter)
	if err != nil {
		return err
	}

	w := printers.GetNewTabWriter(o.Out)
	printer := printers.NewTablePrinter(printers.PrintOptions{
		WithNamespace: true,
		WithKind:      true,
		Wide:          true,
		ShowLabels:    true,
		Kind: schema.GroupKind{
			Group: appcorev1.GroupVersion.Group,
			Kind:  "CliApp",
		},
	})

	w.SetRememberedWidths(nil)
	for i := range apps.Items {
		app := &apps.Items[i]
		printer.PrintObj(app, w)
		w.Flush()
	}

	return nil
}

func newAppListCmd(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *cobra.Command {
	o := &appListOptions{
		GlobalOptions: opts,
		IOStreams:     streams,
		namespace:     metav1.NamespaceAll,
	}

	var cmd = &cobra.Command{
		Use:   "list",
		Short: "List all installed CliApps.",
		Long:  `List all installed CliApps in the cluster.`,
		Example: `# List all CliApps
kubectl dev app list -n app
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

	o.AddPersistentFlags(cmd.Flags())
	return cmd
}
