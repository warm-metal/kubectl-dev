package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/opts"
	"github.com/warm-metal/kubectl-dev/pkg/kubectl"
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"golang.org/x/xerrors"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"
	"k8s.io/client-go/util/retry"
	"strings"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	watchAPI "k8s.io/apimachinery/pkg/watch"
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
		if err := updateDeployEnv(o.clientset, "buildkitd", "buildkitd", o.envs); err != nil {
			return err
		}
	}

	workloads := map[string]runtime.Object{
		"csi-image-warm-metal":      &appsv1.DaemonSet{},
		"buildkitd":                 &appsv1.Deployment{},
		"cliapp-controller-manager": &appsv1.Deployment{},
		"cliapp-session-gate":       &appsv1.Deployment{},
	}

	fmt.Println("Waiting for workloads...")
	wg := sync.WaitGroup{}
	for name, kind := range workloads {
		wg.Add(1)
		go func(name string, objType runtime.Object) {
			defer wg.Done()
			if err := waitForWorkloadToBeReady(context.TODO(), o.clientset, objType, name); err != nil {
				fmt.Fprintf(o.ErrOut, "unable to watch workload %s/%s: %s\n", objType, name, err)
			}
		}(name, kind)
	}

	wg.Wait()
	return nil
}

const appNamespace = "cliapp-system"

func updateDeployEnv(clientset *kubernetes.Clientset, name, container string, envs []corev1.EnvVar) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		client := clientset.AppsV1().Deployments(appNamespace)
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

func waitForWorkloadToBeReady(ctx context.Context, clientset *kubernetes.Clientset, objType runtime.Object, name string) error {
	gvks, _, err := scheme.Scheme.ObjectKinds(objType)
	if err != nil {
		return err
	}

	if len(gvks) == 0 {
		panic(objType)
	}

	listWatcher := cache.NewListWatchFromClient(
		clientset.AppsV1().RESTClient(), strings.ToLower(gvks[0].Kind)+"s",
		appNamespace, fields.OneTermEqualSelector("metadata.name", name),
	)
	_, err = watch.UntilWithSync(ctx, listWatcher, objType, nil, func(event watchAPI.Event) (done bool, err error) {
		switch event.Type {
		case watchAPI.Error:
			st, ok := event.Object.(*metav1.Status)
			if ok {
				err = xerrors.Errorf("failed %s", st.Message)
			} else {
				err = xerrors.Errorf("unknown error:%#v", event.Object)
			}

			return
		case watchAPI.Deleted:
			return
		default:
			switch obj := event.Object.(type) {
			case *appsv1.Deployment:
				if obj.Status.ReadyReplicas == *obj.Spec.Replicas {
					fmt.Println(gvks[0].Kind, name, "is ready")
					done = true
				}
			case *appsv1.DaemonSet:
				if obj.Status.NumberReady == obj.Status.DesiredNumberScheduled {
					fmt.Println(gvks[0].Kind, name, "is ready")
					done = true
				}
			default:
				panic(event.Object.GetObjectKind())
			}
			return
		}
	})

	return err
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
