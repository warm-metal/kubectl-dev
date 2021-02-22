package cmd

import (
	"context"
	"github.com/spf13/cobra"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/opts"
	"github.com/warm-metal/kubectl-dev/pkg/kubectl"
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"strings"
)

type PrepareOptions struct {
	*opts.GlobalOptions
	genericclioptions.IOStreams

	minikube        bool
	useHTTPProxy    bool
	updateManifests bool

	manifestReader io.Reader
	manifestURL    string

	envs      []corev1.EnvVar
	clientset *kubernetes.Clientset
}

func (o *PrepareOptions) Complete(cmd *cobra.Command, args []string) error {
	if o.updateManifests {
		if o.minikube {
			o.manifestURL = latestMinikubeManifests
		} else {
			o.manifestURL = latestContainerdManifests
		}

		return nil
	}

	if o.minikube {
		o.manifestReader = strings.NewReader(minikubeManifests)
	} else {
		o.manifestReader = strings.NewReader(containerdManifests)
	}

	if o.useHTTPProxy {
		proxies, err := utils.GetSysProxy()
		if err != nil {
			return err
		}

		o.envs = proxies
		o.clientset, err = o.ClientSet()
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *PrepareOptions) Validate() error {
	return nil
}

func (o *PrepareOptions) Run() error {
	if len(o.manifestURL) > 0 {
		if err := kubectl.ApplyManifests(o.manifestURL); err != nil {
			return err
		}
	} else {
		if err := kubectl.ApplyManifestsFromStdin(o.manifestReader); err != nil {
			return err
		}
	}

	if len(o.envs) > 0 {
		return updateDeployEnv(o.clientset, "cliapp-buildkitd", "buildkitd", o.envs)
	}

	return nil
}

func updateDeployEnv(clientset *kubernetes.Clientset, name, container string, envs []corev1.EnvVar) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		client := clientset.AppsV1().Deployments("cliapp-system")
		deploy, err := client.Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		for i := range deploy.Spec.Template.Spec.Containers {
			c := &deploy.Spec.Template.Spec.Containers[i]
			if c.Name != container {
				continue
			}

			c.Env = append(c.Env, envs...)
		}

		_, err = client.Update(context.TODO(), deploy, metav1.UpdateOptions{})
		return err
	})
}

func NewCmdPrepare(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *cobra.Command {
	o := PrepareOptions{
		GlobalOptions: opts,
		IOStreams:     streams,
	}

	var cmd = &cobra.Command{
		Use:   "prepare",
		Short: "Install dependencies in the cluster.",
		Example: `# Install cliapp into a cluster.
kubectl dev prepare

# Install cliapp into a minikube cluster.
kubectl dev prepare --minikube

# Install cliapp along with HTTP_PROXY configurations.
kubectl dev prepare --use-proxy

# Install cliapp via the latest remote manifests.
kubectl dev prepare -u
`,
		SilenceUsage: true,
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

	cmd.Flags().BoolVar(&o.minikube, "minikube", o.minikube,
		"If true, the target cluster is assumed to be a minikube cluster.")
	cmd.Flags().BoolVar(&o.useHTTPProxy, "use-proxy", false,
		"If set, use current HTTP proxy settings.")
	cmd.Flags().BoolVarP(&o.updateManifests, "update", "u", o.updateManifests,
		"If true, the latest online manifest will be downloaded.")
	o.AddFlags(cmd.Flags())
	return cmd
}
