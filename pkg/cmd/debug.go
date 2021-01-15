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
	"fmt"
	"github.com/spf13/cobra"
	"github.com/warm-metal/kubectl-dev/pkg/kubectl"
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"golang.org/x/xerrors"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/scale/scheme/extensionsv1beta1"
	"os"
	"strings"
)

const (
	csiDriverName = "csi-image.warm-metal.tech"
)

type DebugOptions struct {
	kubectl.ConfigFlags
	genericclioptions.IOStreams

	debugBaseImage   string
	container        string
	installCSIDriver bool
	minikube         bool

	id          string
	instance    string
	labels      map[string]string
	image       string
	podTmpl     *corev1.PodSpec
	containerID int
	namespace   string
}

func NewDebugOptions(streams genericclioptions.IOStreams) *DebugOptions {
	return &DebugOptions{
		ConfigFlags:    kubectl.NewConfigFlags(),
		IOStreams:      streams,
		debugBaseImage: "bash:5",
		id:             rand.String(5),
		namespace:      metav1.NamespaceDefault,
	}
}

func (o *DebugOptions) Complete(cmd *cobra.Command, args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return xerrors.Errorf("must specify a kind and an object, or an image. See the usage")
	}

	if o.Raw().Namespace != nil && len(*o.Raw().Namespace) > 0 {
		o.namespace = *o.Raw().Namespace
	}

	// FIXME args[0] can be "po/name" other than an image
	if len(args) == 1 {
		o.image = args[0]
		o.instance = fmt.Sprintf("debugger-image-%s", o.id)
		o.labels = map[string]string{
			"debugger.warm-metal.tech/image": "",
		}
		return nil
	}

	o.instance = fmt.Sprintf("debugger-%s-%s-%s", args[0], args[1], o.id)
	o.labels = map[string]string{
		"debugger.warm-metal.tech/kind": args[0],
		"debugger.warm-metal.tech/name": args[1],
	}

	podTmpl, err := o.fetchPod(o.namespace, args)
	if err != nil {
		return err
	}

	if len(podTmpl.Containers) == 1 && len(o.container) == 0 {
		o.podTmpl = podTmpl
		return nil
	}

	if len(o.container) == 0 {
		var containers []string
		for _, c := range podTmpl.Containers {
			containers = append(containers, c.Name)
		}

		return xerrors.Errorf("%#v has more than 1 container. Specify one of %#v via -c", args, containers)
	}

	for i, c := range o.podTmpl.Containers {
		if c.Name == o.container {
			o.podTmpl = podTmpl
			o.containerID = i
			break
		}
	}

	if o.podTmpl == nil {
		return xerrors.Errorf("container %s doesn't found in %#v", o.container, args)
	}

	return nil
}

func (o *DebugOptions) Validate() error {
	if len(o.image) == 0 && o.podTmpl == nil {
		return fmt.Errorf("an image or object is required. See the usage")
	}

	return nil
}

func (o *DebugOptions) loadDefaultTemplate() {
	o.podTmpl = &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "debugger",
				Image: o.image,
				Stdin: true,
			},
		},
	}
}

func (o *DebugOptions) fetchPod(namespace string, kindAndName []string) (*corev1.PodSpec, error) {
	result := resource.NewBuilder(o.Raw()).
		Unstructured().
		ContinueOnError().
		NamespaceParam(namespace).DefaultNamespace().
		ResourceTypeOrNameArgs(true, kindAndName...).
		SingleResourceType().
		Flatten().
		Do()
	if result.Err() != nil {
		return nil, xerrors.Errorf(`can't fetch "%#v": %s`, kindAndName, result.Err())
	}

	infos, err := result.Infos()
	if err != nil {
		return nil, xerrors.Errorf(`can't fetch result of "%#v": %s`, kindAndName, result.Err())
	}

	if len(infos) == 0 {
		return nil, xerrors.Errorf(`no "%#v" found`, kindAndName)
	}

	if len(infos) > 1 {
		panic(infos)
	}

	clientset, err := o.ClientSet()
	if err != nil {
		return nil, err
	}

	var podTmpl *corev1.PodSpec
	ctx := context.TODO()
	opt := metav1.GetOptions{}
	info := infos[0]
	switch info.Mapping.GroupVersionKind.Kind {
	case "Deployment":
		switch info.Mapping.GroupVersionKind.Group {
		case extensionsv1beta1.GroupName:
			deploy, err := clientset.ExtensionsV1beta1().
				Deployments(info.Namespace).
				Get(ctx, info.Name, opt)
			if err != nil {
				return nil, xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, err)
			}

			podTmpl = &deploy.Spec.Template.Spec
		case appsv1.GroupName:
			deploy, err := clientset.AppsV1().Deployments(info.Namespace).Get(ctx, info.Name, opt)
			if err != nil {
				return nil, xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, err)
			}

			podTmpl = &deploy.Spec.Template.Spec
		default:
			panic(info.Mapping.GroupVersionKind)
		}
	case "StatefulSet":
		if info.Mapping.GroupVersionKind.Group != appsv1.GroupName {
			panic(infos[0].Mapping.GroupVersionKind)
		}

		sfst, err := clientset.AppsV1().StatefulSets(info.Namespace).Get(ctx, info.Name, opt)
		if err != nil {
			return nil, xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, err)
		}

		podTmpl = &sfst.Spec.Template.Spec
	case "Job":
		if info.Mapping.GroupVersionKind.Group != batchv1.GroupName {
			panic(infos[0].Mapping.GroupVersionKind)
		}

		job, err := clientset.BatchV1().Jobs(info.Namespace).Get(ctx, info.Name, opt)
		if err != nil {
			return nil, xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, err)
		}

		podTmpl = &job.Spec.Template.Spec
	case "CronJob":
		if info.Mapping.GroupVersionKind.Group != batchv1.GroupName {
			panic(info.Mapping.GroupVersionKind)
		}

		job, err := clientset.BatchV1beta1().CronJobs(info.Namespace).Get(ctx, info.Name, opt)
		if err != nil {
			return nil, xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, err)
		}

		podTmpl = &job.Spec.JobTemplate.Spec.Template.Spec
	case "DaemonSet":
		switch info.Mapping.GroupVersionKind.Group {
		case extensionsv1beta1.GroupName:
			ds, err := clientset.ExtensionsV1beta1().DaemonSets(info.Namespace).Get(ctx, info.Name, opt)
			if err != nil {
				return nil, xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, err)
			}

			podTmpl = &ds.Spec.Template.Spec
		case appsv1.GroupName:
			ds, err := clientset.AppsV1().DaemonSets(info.Namespace).Get(ctx, info.Name, opt)
			if err != nil {
				return nil, xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, err)
			}

			podTmpl = &ds.Spec.Template.Spec
		default:
			panic(info.Mapping.GroupVersionKind)
		}
	case "ReplicaSet":
		switch info.Mapping.GroupVersionKind.Group {
		case extensionsv1beta1.GroupName:
			rs, err := clientset.ExtensionsV1beta1().ReplicaSets(info.Namespace).Get(ctx, info.Name, opt)
			if err != nil {
				return nil, xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, err)
			}

			podTmpl = &rs.Spec.Template.Spec
		case appsv1.GroupName:
			rs, err := clientset.AppsV1().ReplicaSets(info.Namespace).Get(ctx, info.Name, opt)
			if err != nil {
				return nil, xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, err)
			}

			podTmpl = &rs.Spec.Template.Spec
		default:
			panic(info.Mapping.GroupVersionKind)
		}
	case "Pod":
		if info.Mapping.GroupVersionKind.Group != corev1.GroupName {
			panic(info.Mapping.GroupVersionKind)
		}

		pod, err := clientset.CoreV1().Pods(info.Namespace).Get(ctx, info.Name, opt)
		if err != nil {
			return nil, xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, err)
		}

		podTmpl = &pod.Spec
	default:
		err = fmt.Errorf("%s not found. Forgot a namespace? ", strings.Join(kindAndName, " "))
		return nil, xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, err)
	}

	return podTmpl, nil
}

const (
	manifestMinikube   = "https://raw.githubusercontent.com/warm-metal/csi-driver-image/master/install/cri-containerd-minikube.yaml"
	manifestContainerd = "https://raw.githubusercontent.com/warm-metal/csi-driver-image/master/install/cri-containerd.yaml"
)

func (o *DebugOptions) Run() error {
	if o.installCSIDriver {
		manifest := manifestContainerd
		if o.minikube {
			manifest = manifestMinikube
		}

		fmt.Println("Install CSI driver for image mounting...")
		fmt.Printf(`use manifests "%s"\n`, manifest)
		fmt.Println("clear all installed objects")
		if err := kubectl.DeleteManifests(manifest); err != nil {
			return err
		}

		fmt.Println("install manifests")
		if err := kubectl.ApplyManifests(manifest); err != nil {
			return err
		}
	}

	if len(o.image) > 0 {
		if o.podTmpl != nil {
			panic(o.podTmpl.String())
		}

		o.loadDefaultTemplate()
	} else {
		o.image = o.podTmpl.Containers[o.containerID].Image
	}

	container := &o.podTmpl.Containers[o.containerID]
	container.Image = o.debugBaseImage
	args := append(container.Command, container.Args...)
	container.Command = nil
	container.Args = []string{"tail", "-f", "/dev/null"}
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "IMAGE_ARGS",
		Value: strings.Join(args, " "),
	})
	container.Stdin = true
	container.ReadinessProbe = nil
	container.LivenessProbe = nil
	container.StartupProbe = nil

	volume := corev1.Volume{
		Name: fmt.Sprintf("debugger-image-%s", o.id),
		VolumeSource: corev1.VolumeSource{
			CSI: &corev1.CSIVolumeSource{
				Driver: csiDriverName,
				VolumeAttributes: map[string]string{
					"image": o.image,
				},
			},
		},
	}

	o.podTmpl.Volumes = append(o.podTmpl.Volumes, volume)
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      volume.Name,
		ReadOnly:  true,
		MountPath: "/image",
	})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   o.instance,
			Labels: o.labels,
		},
		Spec: *o.podTmpl,
	}

	clientset, err := o.ClientSet()
	if err != nil {
		return err
	}

	fmt.Printf("Create debugger Pod %s...\n", pod.Name)
	newPod, err := clientset.CoreV1().Pods(o.namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	fmt.Println("waiting Pod ready")
	watcher, err := clientset.CoreV1().Pods(o.namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: fields.Set{"metadata.name": newPod.Name}.AsSelector().String(),
		Watch:         true,
	})

	if err != nil {
		return err
	}

	err = utils.WaitUntilErrorOr(watcher, func(object runtime.Object) (b bool, err error) {
		fmt.Printf("Debugger %s updated\n", newPod.Name)
		return object.(*corev1.Pod).Status.Phase == corev1.PodRunning, nil
	})

	if err != nil {
		return xerrors.Errorf("can't start debugger Pod %s: %s", newPod.Name, err)
	}

	fmt.Printf("Debugger Pod %s is ready\n", newPod.Name)

	defer func() {
		err = clientset.CoreV1().Pods(o.namespace).Delete(context.TODO(), newPod.Name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "can't delete the pod debugger: %s\n", err)
		}
	}()

	fmt.Printf("Start bash of debugger Pod %s\n", newPod.Name)
	return kubectl.Exec(newPod.Name, newPod.Namespace, container.Name, "bash")
}

func NewCmdDebug(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDebugOptions(streams)

	var cmd = &cobra.Command{
		Use:   "debug",
		Short: "Debug running or failed workloads or images",
		Long: `The image of the target workload will be mounted to a new Pod. You will see all original configurations 
even the filesystem in the new Pod, except the same entrypoint.
`,
		Example: `#Debug a running workload whether if failed or not.
kubectl dev debug deploy name

# 
kubectl dev debug pod name

kubectl dev debug ds name

kubectl dev debug job name

kubectl dev debug cronjob name

kubectl dev debug sts name

kubectl dev debug rs name

#Debug an image.
kubectl dev debug image-name
`,
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
	cmd.Flags().StringVar(
		&o.debugBaseImage, "base", o.debugBaseImage,
		`Base image used to mount the target image. "bash" is required in the base image`)
	cmd.Flags().StringVarP(
		&o.container, "container", "c", o.container,
		"Container of the specified object if in which there are multiple containers")
	cmd.Flags().BoolVar(&o.installCSIDriver, "also-apply-csi-driver", false,
		`Apply the CSI driver "https://github.com/warm-metal/csi-driver-image". The driver is required. If you already have it installed, turn it off.`)

	o.AddFlags(cmd.Flags())

	return cmd
}
