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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"net/url"
	"sigs.k8s.io/yaml"
)

var readOnly = true
var privileged = true
var numReplicas int32 = 1

type BuilderInstallOptions struct {
	configFlags *genericclioptions.ConfigFlags

	reinstall          bool
	printManifest      bool
	customManifestFile string

	minikube        bool
	minikubeProfile string

	ContainerdAddr       string
	ContainerdSocketPath string
	containerdRoot       string

	Port      int
	namespace string
}

func newBuilderInstallOptions(streams genericclioptions.IOStreams) *BuilderInstallOptions {
	return &BuilderInstallOptions{
		configFlags: genericclioptions.NewConfigFlags(true),
		namespace:   "dev",
	}
}

func (o *BuilderInstallOptions) Complete(cmd *cobra.Command, args []string) error {
	if o.minikube {
		o.containerdRoot = "/mnt/vda1/var/lib/containerd"
	}

	addr, err := url.Parse(o.ContainerdAddr)
	if err != nil {
		return err
	}

	if addr.Scheme != "unix" {
		return fmt.Errorf("containerd endpoint should be a unix socket")
	}

	o.ContainerdSocketPath = addr.Path
	return nil
}

func (o *BuilderInstallOptions) Validate() error {
	return nil
}

func (o *BuilderInstallOptions) Run() error {
	cm := o.genBuildkitdToml()
	svc, deploy := o.genBuildkitdWorkload()

	if o.printManifest {
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

	config, err := o.configFlags.ToRESTConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	if _, err = clientset.CoreV1().Namespaces().Get(context.TODO(), o.namespace, metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

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
		if err := kubectl.Delete("svc", svc.Name, o.namespace); err != nil && !errors.IsNotFound(err) {
			return err
		}
		if err := kubectl.Delete("deploy", deploy.Name, o.namespace); err != nil && !errors.IsNotFound(err) {
			return err
		}
		if err := kubectl.Delete("cm", cm.Name, o.namespace); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	opts := metav1.CreateOptions{}
	_, err = clientset.CoreV1().ConfigMaps(o.namespace).Create(context.TODO(), cm, opts)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	_, err = clientset.CoreV1().Services(o.namespace).Create(context.TODO(), svc, opts)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	_, err = clientset.AppsV1().Deployments(o.namespace).Create(context.TODO(), deploy, opts)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func newCmdInstall(streams genericclioptions.IOStreams) *cobra.Command {
	o := newBuilderInstallOptions(streams)

	var cmd = &cobra.Command{
		Use:   "install",
		Short: "Install builder in k8s cluster.",
		Long: `Install buildkitd in the cluster that, "kubectl-dev build" can use to build images.
Buildkitd will share the same image store with the runtime.`,
		Example: ``,
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
	cmd.Flags().BoolVar(&o.printManifest, "print-manifest", false,
		"If true, print manifests of the buildkitd.")
	cmd.Flags().StringVarP(&o.customManifestFile, "file", "f", "",
		"Custom manifest of the buildkitd.")

	cmd.Flags().BoolVar(&o.minikube, "minikube", o.minikube,
		"If true, the target cluster is assumed to be a minikube cluster.")
	cmd.Flags().StringVar(&o.minikubeProfile, "minikube-profile", "minikube",
		"Profile ID of the target minikube cluster.")

	cmd.Flags().StringVar(&o.ContainerdAddr, "containerd-addr", "unix:///run/containerd/containerd.sock",
		"The containerd socket address. Must be a valid URL of a UNIX socket.")
	cmd.Flags().StringVar(&o.containerdRoot, "containerd-root", "/var/lib/containerd",
		"The root path of containerd.")

	cmd.Flags().IntVar(&o.Port, "port", 1234, "Port of buildkit")

	o.configFlags.AddFlags(cmd.Flags())

	return cmd
}
