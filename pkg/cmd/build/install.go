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
	"github.com/spf13/cobra"
	"github.com/warm-metal/kubectl-dev/pkg/kubectl"
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"golang.org/x/xerrors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/yaml"
	"strings"
)

var readOnly = true
var privileged = true
var numReplicas int32 = 1

const builderNamespace = "dev"

type BuilderInstallOptions struct {
	kubectl.ConfigFlags

	reinstall          bool
	printManifests     bool
	customManifestFile string
	useHTTPProxy       bool

	minikube        bool
	minikubeProfile string

	ContainerdRuntimeRoot string
	containerdRoot        string

	Port      int
	namespace string
}

func newBuilderInstallOptions(streams genericclioptions.IOStreams) *BuilderInstallOptions {
	return &BuilderInstallOptions{
		ConfigFlags: kubectl.NewConfigFlags(),
		namespace:   builderNamespace,
	}
}

func (o *BuilderInstallOptions) Complete(cmd *cobra.Command, args []string) error {
	if o.minikube {
		o.containerdRoot = "/mnt/vda1/var/lib/containerd"
	}

	return nil
}

func (o *BuilderInstallOptions) Validate() error {
	return nil
}

func (o *BuilderInstallOptions) Run() error {
	cm := o.genBuildkitdToml()
	svc, deploy, err := o.genBuildkitdWorkload()
	if err != nil {
		return err
	}

	if o.printManifests {
		j, err := json.Marshal([]runtime.Object{cm, svc, deploy})
		if err != nil {
			panic(err)
		}

		y, err := yaml.JSONToYAML(j)
		if err != nil {
			panic(err)
		}

		fmt.Printf("%s\n", string(y))
		return nil
	}

	clientset, err := o.ClientSet()
	if err != nil {
		return err
	}

	if _, err = clientset.CoreV1().Namespaces().Get(context.TODO(), o.namespace, metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		fmt.Println("Create namespace", o.namespace, "...")
		_, err = clientset.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: o.namespace,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	if o.reinstall {
		fmt.Println("Reinstall buildkitd...")
		fmt.Println("delete Service", svc.Name)
		if err := kubectl.Delete("svc", svc.Name, o.namespace); err != nil && !errors.IsNotFound(err) {
			return err
		}

		fmt.Println("delete Deployment", deploy.Name)
		if err := kubectl.Delete("deploy", deploy.Name, o.namespace); err != nil && !errors.IsNotFound(err) {
			return err
		}

		fmt.Println("delete ConfigMap", cm.Name)
		if err := kubectl.Delete("cm", cm.Name, o.namespace); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	opts := metav1.CreateOptions{}

	fmt.Println("create ConfigMap", cm.Name)
	_, err = clientset.CoreV1().ConfigMaps(o.namespace).Create(context.TODO(), cm, opts)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	fmt.Println("create Service", svc.Name)
	_, err = clientset.CoreV1().Services(o.namespace).Create(context.TODO(), svc, opts)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	fmt.Println("create Deployment", deploy.Name)
	_, err = clientset.AppsV1().Deployments(o.namespace).Create(context.TODO(), deploy, opts)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	fmt.Println("wait Deployment...")
	watcher, err := clientset.AppsV1().Deployments(o.namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: fields.Set{"metadata.name": deploy.Name}.AsSelector().String(),
		Watch:         true,
	})

	if err != nil {
		return err
	}

	err = utils.WaitUntilErrorOr(watcher, func(object runtime.Object) (b bool, err error) {
		deploy := object.(*appsv1.Deployment)
		fmt.Printf("available replicas %d/%d\n", deploy.Status.AvailableReplicas, *deploy.Spec.Replicas)
		return *deploy.Spec.Replicas == deploy.Status.AvailableReplicas, nil
	})

	if err != nil {
		return xerrors.Errorf("can't start Deployment buildkitd: %s", err)
	}

	addrs, err := fetchBuilderEndpoints(clientset)
	if err != nil {
		return err
	}

	if len(addrs) > 0 {
		fmt.Println("Installed. Builder works on any of", strings.Join(addrs, ","))
	} else {
		fmt.Println("Installed. Builder works on", addrs[0])
	}

	return nil
}

func newCmdInstall(streams genericclioptions.IOStreams) *cobra.Command {
	o := newBuilderInstallOptions(streams)

	var cmd = &cobra.Command{
		Use:   "install",
		Short: "Install buildkitd in a k8s cluster.",
		Long: `Install buildkitd in the cluster that, "kubectl-dev build" can use to build images.
Buildkitd will share the same image store with the runtime.`,
		Example: `# Install build toolchains.
kubectl dev build install

# Install build toolchains in a minikube cluster.
kubectl dev build install --minikube

# Show manifests
kubectl dev build install --print-manifests

# Customize containerd configuration.
kubectl dev build install --containerd-addr=unix://foo --containerd-root=bar
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

	cmd.Flags().BoolVar(&o.reinstall, "reinstall", false, "Override the previous install")
	cmd.Flags().BoolVar(&o.printManifests, "print-manifests", false,
		"If true, print manifests of the buildkitd.")
	cmd.Flags().StringVarP(&o.customManifestFile, "file", "f", "",
		"Custom manifest of the buildkitd.")

	cmd.Flags().BoolVar(&o.minikube, "minikube", o.minikube,
		"If true, the target cluster is assumed to be a minikube cluster.")
	cmd.Flags().StringVar(&o.minikubeProfile, "minikube-profile", "minikube",
		"[NOT SUPPORTED YET]Profile ID of the target minikube cluster.")

	cmd.Flags().StringVar(&o.ContainerdRuntimeRoot, "containerd-runtime-root", "/run/containerd",
		`The runtime root path of containerd. It usually is "/run/containerd". containerd.sock must be in the directory.`)
	cmd.Flags().StringVar(&o.containerdRoot, "containerd-root", "/var/lib/containerd",
		"The root path of containerd.")

	cmd.Flags().IntVar(&o.Port, "port", 1234, "Port of buildkit")
	cmd.Flags().BoolVar(&o.useHTTPProxy, "use-proxy", false,
		"If set, use current HTTP proxy settings.")

	o.AddFlags(cmd.Flags())
	return cmd
}
