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
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/rand"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/scale/scheme/extensionsv1beta1"
	"k8s.io/client-go/tools/clientcmd/api"
	"os"
	"strings"
)

const (
	csiDriverName = "csi-image.warm-metal.tech"
)

type DebugOptions struct {
	configFlags *genericclioptions.ConfigFlags
	genericclioptions.IOStreams
	rawConfig api.Config

	debugBaseImage   string
	container        string
	installCSIDriver bool

	id          string
	instance    string
	labels      map[string]string
	image       string
	podTmpl     *corev1.PodSpec
	containerID int
}

func NewDebugOptions(streams genericclioptions.IOStreams) *DebugOptions {
	return &DebugOptions{
		configFlags:    genericclioptions.NewConfigFlags(true),
		IOStreams:      streams,
		debugBaseImage: "bash:5",
		id:             rand.String(5),
	}
}

func (o *DebugOptions) Complete(cmd *cobra.Command, args []string) error {
	var err error
	o.rawConfig, err = o.configFlags.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return err
	}

	if len(args) < 1 || len(args) > 2 {
		return fmt.Errorf("must specify a kind and an object, or an image. See the usage")
	}

	if o.installCSIDriver {
		const manifest = "https://raw.githubusercontent.com/warm-metal/csi-driver-image/master/install/cri-containerd-minikube.yaml"
		if err := kubectl.DeleteManifests(manifest); err != nil {
			return err
		}

		if err := kubectl.ApplyManifests(manifest); err != nil {
			return err
		}
	}

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

	result := resource.NewBuilder(o.configFlags).
		Unstructured().
		ContinueOnError().
		NamespaceParam(*o.configFlags.Namespace).DefaultNamespace().
		ResourceTypeOrNameArgs(true, args...).
		SingleResourceType().
		Flatten().
		Do()
	if result.Err() != nil {
		cmd.PrintErrln(err.Error())
		return err
	}

	infos, err := result.Infos()
	if err != nil {
		cmd.PrintErrln(err.Error())
		return err
	}

	if len(infos) == 0 {
		err = fmt.Errorf("%s not found", strings.Join(args, " "))
		cmd.PrintErrf(err.Error())
		return err
	}

	if len(infos) > 1 {
		panic(infos)
	}

	config, err := o.configFlags.ToRESTConfig()
	if err != nil {
		cmd.PrintErrln(err.Error())
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		cmd.PrintErrln(err.Error())
		return err
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
				cmd.PrintErrln(err.Error())
				return err
			}

			podTmpl = &deploy.Spec.Template.Spec
		case appsv1.GroupName:
			deploy, err := clientset.AppsV1().Deployments(info.Namespace).Get(ctx, info.Name, opt)
			if err != nil {
				cmd.PrintErrln(err.Error())
				return err
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
			cmd.PrintErrln(err.Error())
			return err
		}

		podTmpl = &sfst.Spec.Template.Spec
	case "Job":
		if info.Mapping.GroupVersionKind.Group != batchv1.GroupName {
			panic(infos[0].Mapping.GroupVersionKind)
		}

		job, err := clientset.BatchV1().Jobs(info.Namespace).Get(ctx, info.Name, opt)
		if err != nil {
			cmd.PrintErrln(err.Error())
			return err
		}

		podTmpl = &job.Spec.Template.Spec
	case "CronJob":
		if info.Mapping.GroupVersionKind.Group != batchv1.GroupName {
			panic(info.Mapping.GroupVersionKind)
		}

		job, err := clientset.BatchV1beta1().CronJobs(info.Namespace).Get(ctx, info.Name, opt)
		if err != nil {
			cmd.PrintErrln(err.Error())
			return err
		}

		podTmpl = &job.Spec.JobTemplate.Spec.Template.Spec
	case "DaemonSet":
		switch info.Mapping.GroupVersionKind.Group {
		case extensionsv1beta1.GroupName:
			ds, err := clientset.ExtensionsV1beta1().DaemonSets(info.Namespace).Get(ctx, info.Name, opt)
			if err != nil {
				cmd.PrintErrln(err.Error())
				return err
			}

			podTmpl = &ds.Spec.Template.Spec
		case appsv1.GroupName:
			ds, err := clientset.AppsV1().DaemonSets(info.Namespace).Get(ctx, info.Name, opt)
			if err != nil {
				cmd.PrintErrln(err.Error())
				return err
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
				cmd.PrintErrln(err.Error())
				return err
			}

			podTmpl = &rs.Spec.Template.Spec
		case appsv1.GroupName:
			rs, err := clientset.AppsV1().ReplicaSets(info.Namespace).Get(ctx, info.Name, opt)
			if err != nil {
				cmd.PrintErrln(err.Error())
				return err
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
			cmd.PrintErrln(err.Error())
			return err
		}

		podTmpl = &pod.Spec
	default:
		err = fmt.Errorf("%s not found. Forgot a namespace? ", strings.Join(args, " "))
		cmd.PrintErrf(err.Error())
		return err
	}

	if len(podTmpl.Containers) == 1 && len(o.container) == 0 {
		o.podTmpl = podTmpl
		return nil
	}

	if len(o.container) == 0 {
		err = fmt.Errorf("%s has more than 1 container. A specific container is required. See the usage",
			strings.Join(args, " "))
		cmd.PrintErrf(err.Error())
		return err
	}

	for i, c := range o.podTmpl.Containers {
		if c.Name == o.container {
			o.podTmpl = podTmpl
			o.containerID = i
			break
		}
	}

	if o.podTmpl == nil {
		err = fmt.Errorf("%s has no container named %s", strings.Join(args, " "), o.container)
		cmd.PrintErrf(err.Error())
		return err
	}

	return nil
}

func (o *DebugOptions) Validate() error {
	if len(o.image) == 0 && o.podTmpl == nil {
		err := fmt.Errorf("an image or object is required. See the usage")
		return err
	}

	return nil
}

func (o *DebugOptions) loadDefaultTemplate() error {
	o.podTmpl = &corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:  "debugger",
				Image: o.image,
				Stdin: true,
				TTY:   true,
			},
		},
	}
	return nil
}

func (o *DebugOptions) Run() error {
	config, err := o.configFlags.ToRESTConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	if len(o.image) > 0 {
		if o.podTmpl != nil {
			panic(o.podTmpl.String())
		}

		if err := o.loadDefaultTemplate(); err != nil {
			return err
		}
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

	ns := metav1.NamespaceDefault
	if o.configFlags.Namespace != nil && len(*o.configFlags.Namespace) > 0 {
		ns = *o.configFlags.Namespace
	}

	newPod, err := clientset.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	watcher, err := clientset.CoreV1().Pods(ns).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: fields.Set{"metadata.name": newPod.Name}.AsSelector().String(),
		Watch:         true,
	})

	if err != nil {
		return err
	}

	for {
		ev := <-watcher.ResultChan()

		if ev.Type != watch.Modified && ev.Type != watch.Added {
			watcher.Stop()
			return fmt.Errorf("the new pod can start, %#v", ev)
		}

		if ev.Object.(*corev1.Pod).Status.Phase == corev1.PodRunning {
			break
		}
	}

	watcher.Stop()

	defer func() {
		if err = clientset.CoreV1().Pods(ns).Delete(context.TODO(), newPod.Name, metav1.DeleteOptions{}); err != nil {
			fmt.Fprintf(os.Stderr, "can't delete the pod debugger: %s\n", err)
		}
	}()

	return kubectl.Exec(newPod.Name, newPod.Namespace, container.Name, "bash")
}

func NewCmdDebug(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDebugOptions(streams)

	var debugCmd = &cobra.Command{
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

	debugCmd.Flags().StringVar(
		&o.debugBaseImage, "base", o.debugBaseImage,
		`Base image used to mount the target image. "bash" is required in the base image`)
	debugCmd.Flags().StringVarP(
		&o.container, "container", "c", o.container,
		"Container of the specified object if in which there are multiple containers")
	debugCmd.Flags().BoolVar(&o.installCSIDriver, "also-apply-csi-driver", false,
		`Apply the CSI driver "https://github.com/warm-metal/csi-driver-image". The driver is required. If you already have it installed, turn it off.`)

	o.configFlags.AddFlags(debugCmd.Flags())

	return debugCmd
}
