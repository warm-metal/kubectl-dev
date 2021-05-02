package cmd

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	appcorev1 "github.com/warm-metal/cliapp/pkg/apis/cliapp/v1"
	configv1 "github.com/warm-metal/cliapp/pkg/apis/config/v1"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/opts"
	"github.com/warm-metal/kubectl-dev/pkg/kubectl"
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"golang.org/x/xerrors"
	"io"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/watch"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/yaml"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	watchAPI "k8s.io/apimachinery/pkg/watch"
)

type PrepareOptions struct {
	*opts.GlobalOptions
	genericclioptions.IOStreams

	defaultShell          string
	defaultDistro         string
	defaultAppContetImage string
	idleLivesLast         time.Duration

	minikube        bool
	useHTTPProxy    bool
	updateManifests bool
	builderEnvs     []string

	manifestReader io.Reader
	manifestURL    string

	envs      []corev1.EnvVar
	clientset *kubernetes.Clientset

	shellRCFile string
	shellRC     string
}

func (o *PrepareOptions) Complete(cmd *cobra.Command, args []string) (err error) {
	o.clientset, err = o.ClientSet()
	if err != nil {
		return err
	}

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
	}

	for _, envs := range o.builderEnvs {
		if len(envs) == 0 {
			continue
		}

		kv := strings.Split(envs, "=")
		o.envs = append(o.envs, corev1.EnvVar{
			Name:  kv[0],
			Value: strings.Join(kv[1:], "="),
		})
	}

	if len(o.shellRCFile) > 0 {
		if len(o.defaultShell) == 0 {
			return xerrors.New("default shell is required for the rc file")
		}

		rc, err := ioutil.ReadFile(utils.ExpandTilde(o.shellRCFile))
		if err != nil {
			return err
		}

		o.shellRC = string(rc)
	}

	return nil
}

func (o *PrepareOptions) Validate() error {
	return nil
}

func (o *PrepareOptions) Run(ctx context.Context) error {
	if len(o.manifestURL) > 0 {
		if err := kubectl.ApplyManifests(o.manifestURL); err != nil {
			return err
		}
	} else {
		if err := kubectl.ApplyManifestsFromStdin(o.manifestReader); err != nil {
			return err
		}
	}

	workloads := map[string]runtime.Object{
		"csi-image-warm-metal":      &appsv1.DaemonSet{},
		"csi-configmap-warm-metal":  &appsv1.DaemonSet{},
		"buildkitd":                 &appsv1.Deployment{},
		"cliapp-controller-manager": &appsv1.Deployment{},
		"cliapp-session-gate":       &appsv1.Deployment{},
	}

	err := fetchObject(ctx, o.clientset, workloads["cliapp-controller-manager"], "cliapp-controller-manager")
	if err != nil {
		return err
	}

	if len(o.envs) > 0 {
		err := updateDeployEnv(ctx, o.clientset, "buildkitd", "buildkitd", o.envs, !o.useHTTPProxy)
		if err != nil {
			return err
		}
	}

	if len(o.defaultShell) > 0 && len(o.shellRC) > 0 {
		err := updateShellRC(ctx, o.clientset, o.defaultShell, o.shellRC)
		if err != nil {
			return err
		}
	}

	if err = updateDefaultConfiguration(
		ctx, o.clientset, o.defaultShell, o.defaultDistro, o.defaultAppContetImage, o.idleLivesLast,
	); err != nil {
		return err
	}

	fmt.Println("Waiting for workloads...")
	wg := sync.WaitGroup{}
	for name, kind := range workloads {
		wg.Add(1)
		go func(name string, objType runtime.Object) {
			defer wg.Done()
			if err := waitForWorkloadsToBeReady(ctx, o.clientset, objType, name); err != nil {
				fmt.Fprintf(o.ErrOut, "unable to watch workload %s: %s\n", name, err)
			}
		}(name, kind)
	}

	wg.Wait()
	return nil
}

const appNamespace = "cliapp-system"

func updateShellRC(
	ctx context.Context, clientset *kubernetes.Clientset, shell, rc string,
) error {
	key := ""
	switch appcorev1.CliAppShell(shell) {
	case appcorev1.CliAppShellBash:
		key = ".bash_profile"
	case appcorev1.CliAppShellZsh:
		key = ".zshrc"
	default:
		return xerrors.Errorf("the default shell must be either %q or %q", appcorev1.CliAppShellBash,
			appcorev1.CliAppShellZsh)
	}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cm, err := clientset.CoreV1().ConfigMaps(appNamespace).Get(ctx, "cliapp-shell-context", metav1.GetOptions{})
		if err != nil {
			return err
		}

		cm.Data[key] = rc
		_, err = clientset.CoreV1().ConfigMaps(appNamespace).Update(ctx, cm, metav1.UpdateOptions{})
		return err
	})
}

func updateDeployEnv(
	ctx context.Context, clientset *kubernetes.Clientset, name, container string, newEnvs []corev1.EnvVar, cleanProxy bool,
) error {
	cleanEnvMap := map[string]bool{}
	if cleanProxy {
		cleanEnvMap = map[string]bool{
			"http_proxy":  true,
			"HTTP_PROXY":  true,
			"https_proxy": true,
			"HTTPS_PROXY": true,
			"no_proxy":    true,
			"NO_PROXY":    true,
		}
	}

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		client := clientset.AppsV1().Deployments(appNamespace)
		deploy, err := client.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		newEvnMap := make(map[string]int, len(newEnvs))
		for i := range newEnvs {
			newEvnMap[newEnvs[i].Name] = i
		}

		for i := range deploy.Spec.Template.Spec.Containers {
			c := &deploy.Spec.Template.Spec.Containers[i]
			if c.Name != container {
				continue
			}

			envs := make([]corev1.EnvVar, 0, len(c.Env)+len(newEnvs))

			for i := range c.Env {
				env := &c.Env[i]
				if cleanEnvMap[env.Name] {
					continue
				}

				if updated, found := newEvnMap[env.Name]; found {
					delete(newEvnMap, env.Name)
					envs = append(envs, newEnvs[updated])
					continue
				}

				envs = append(envs, *env)
			}

			for _, i := range newEvnMap {
				envs = append(envs, newEnvs[i])
			}

			c.Env = envs
			break
		}

		_, err = client.Update(ctx, deploy, metav1.UpdateOptions{})
		return err
	})
}

func updateDefaultConfiguration(
	ctx context.Context, clientset *kubernetes.Clientset,
	defaultShell, defaultDistro, defaultAppContetImage string, idleLivesLast time.Duration,
) error {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		cm, err := clientset.CoreV1().ConfigMaps(appNamespace).Get(ctx, "cliapp-manager-config", metav1.GetOptions{})
		if err != nil {
			return err
		}

		content := cm.Data["controller_manager_config.yaml"]
		y, err := yaml.YAMLToJSON([]byte(content))
		if err != nil {
			return err
		}

		conf := configv1.CliAppDefault{}
		if err = yaml.Unmarshal(y, &conf); err != nil {
			return err
		}

		conf.DefaultShell = defaultShell
		conf.DefaultDistro = defaultDistro
		conf.DefaultAppContextImage = defaultAppContetImage
		conf.DurationIdleLivesLast = metav1.Duration{idleLivesLast}
		y, err = yaml.Marshal(&conf)
		if err != nil {
			return err
		}

		cm.Data["controller_manager_config.yaml"] = string(y)
		_, err = clientset.CoreV1().ConfigMaps(appNamespace).Update(ctx, cm, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		return err
	}

	deploy, err := clientset.AppsV1().Deployments(appNamespace).Get(ctx, "cliapp-controller-manager", metav1.GetOptions{})
	if err != nil {
		return err
	}

	return clientset.CoreV1().Pods(appNamespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(deploy.Spec.Selector.MatchLabels).String(),
	})
}

func waitForWorkloadsToBeReady(ctx context.Context, clientset *kubernetes.Clientset, objType runtime.Object, name string) error {
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

	accessor, err := meta.CommonAccessor(objType)
	if err != nil {
		panic(err)
	}

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
				if obj.ResourceVersion != accessor.GetResourceVersion() &&
					obj.Status.ReadyReplicas == *obj.Spec.Replicas {
					fmt.Println(gvks[0].Kind, name, "(ver."+obj.ResourceVersion+")", "is ready")
					done = true
				}
			case *appsv1.DaemonSet:
				if obj.ResourceVersion != accessor.GetResourceVersion() &&
					obj.Status.NumberReady == obj.Status.DesiredNumberScheduled {
					fmt.Println(gvks[0].Kind, name, "(ver."+obj.ResourceVersion+")", "is ready")
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

func fetchObject(ctx context.Context, clientset *kubernetes.Clientset, obj runtime.Object, name string) error {
	gvks, _, err := scheme.Scheme.ObjectKinds(obj)
	if err != nil {
		return err
	}

	if len(gvks) == 0 {
		panic(obj)
	}

	return clientset.AppsV1().RESTClient().Get().
		Namespace(appNamespace).
		Resource(strings.ToLower(gvks[0].Kind)+"s").
		VersionedParams(&metav1.GetOptions{}, metav1.ParameterCodec).
		Name(name).
		Do(ctx).
		Into(obj)
}

func NewCmdPrepare(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *cobra.Command {
	o := PrepareOptions{
		GlobalOptions: opts,
		IOStreams:     streams,
		idleLivesLast: 10 * time.Minute,
		defaultShell:  string(appcorev1.CliAppShellBash),
		defaultDistro: string(appcorev1.CliAppDistroAlpine),
	}

	var cmd = &cobra.Command{
		Use:   "prepare",
		Short: "Install dependencies in the cluster.",
		Example: `# Install cliapp into a cluster.
kubectl dev prepare

# Install cliapp into a minikube cluster.
kubectl dev prepare --minikube

# Install cliapp and set local HTTP_PROXY settings to buildkit.
kubectl dev prepare --use-proxy

# Install cliapp and set the environment variable to buildkit.
kubectl dev prepare --builder-env GOPROXY='https://goproxy.cn|https://goproxy.io|direct'

# Install cliapp via the latest remote manifests.
kubectl dev prepare -u

# Install cliapp w/o custom shell and distro
kubectl dev prepare --minikube --distro ubuntu --shell zsh
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

	cmd.Flags().BoolVar(&o.minikube, "minikube", o.minikube,
		"If true, the target cluster is assumed to be a minikube cluster.")
	cmd.Flags().BoolVar(&o.useHTTPProxy, "use-proxy", false,
		"If set, use current HTTP proxy settings.")
	cmd.Flags().BoolVarP(&o.updateManifests, "update", "u", o.updateManifests,
		"If true, the latest online manifest will be downloaded.")
	cmd.Flags().StringSliceVar(&o.builderEnvs, "builder-env", nil,
		"Set environment variables for buildkit. Such as setting GOPROXY=goproxy.cn for go module.")
	cmd.Flags().StringVar(&o.defaultDistro, "distro", o.defaultDistro,
		"Linux distro that the app prefer. The default value is alpine. ubuntu is also supported.")
	cmd.Flags().StringVar(&o.defaultShell, "shell", o.defaultShell,
		"The shell you prefer. The default value is bash. You can also use zsh instead.")
	cmd.Flags().StringVar(&o.defaultAppContetImage, "app-context-image", "",
		"The context image to start an app. It must have the default shell installed.")
	cmd.Flags().DurationVar(&o.idleLivesLast, "idle-lives-last", o.idleLivesLast,
		"Duration in that the background pod would be still alive even no active session opened.")
	cmd.Flags().StringVar(&o.shellRCFile, "shell-rc", o.shellRCFile,
		"Use a local shell resource file(~/.zshrc or ~/.bash_profile) in interactive cliapps or debuggers.")

	o.AddFlags(cmd.Flags())
	return cmd
}
