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
	"encoding/base64"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/warm-metal/kubectl-dev/pkg/kubectl"
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"golang.org/x/xerrors"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/scale/scheme/extensionsv1beta1"
	"net"
	"net/url"
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
	createNew        bool
	debuggerPodName  string
	keepDebugger     bool
	useHTTPProxy     bool

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
	if o.Raw().Namespace != nil && len(*o.Raw().Namespace) > 0 {
		o.namespace = *o.Raw().Namespace
	}

	if len(args) == 0 {
		if len(o.image) == 0 {
			return xerrors.Errorf("specify an image or an object")
		}

		encodedImage := base64.StdEncoding.EncodeToString([]byte(o.image))
		o.instance = fmt.Sprintf("debugger-%s-%s", encodedImage, o.id)
		o.labels = map[string]string{
			"debugger.warm-metal.tech/image": encodedImage,
		}

		return nil
	}

	kind, name, podTmpl, err := o.fetchPod(o.namespace, args)
	if err != nil {
		return err
	}

	o.instance = fmt.Sprintf("debugger-%s-%s-%s", kind, name, o.id)
	o.labels = map[string]string{
		"debugger.warm-metal.tech/kind":      kind,
		"debugger.warm-metal.tech/name":      name,
		"debugger.warm-metal.tech/container": o.container,
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

		return xerrors.Errorf("%s/%s has more than 1 container. Specify one of %#v via -c",
			kind, name, containers)
	}

	for i, c := range o.podTmpl.Containers {
		if c.Name == o.container {
			o.podTmpl = podTmpl
			o.containerID = i
			break
		}
	}

	if o.podTmpl == nil {
		return xerrors.Errorf("container %s doesn't found in %s/%s", o.container, kind, name)
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

func (o *DebugOptions) fetchPod(
	namespace string, kindAndName []string,
) (kind, name string, podTmpl *corev1.PodSpec, err error) {
	result := resource.NewBuilder(o.Raw()).
		Unstructured().
		ContinueOnError().
		NamespaceParam(namespace).DefaultNamespace().
		ResourceTypeOrNameArgs(true, kindAndName...).
		SingleResourceType().
		Flatten().
		Do()
	if result.Err() != nil {
		err = xerrors.Errorf(`can't fetch "%#v": %s`, kindAndName, result.Err())
		return
	}

	infos, err := result.Infos()
	if err != nil {
		err = xerrors.Errorf(`can't fetch result of "%#v": %s`, kindAndName, result.Err())
		return
	}

	if len(infos) == 0 {
		err = xerrors.Errorf(`no "%#v" found`, kindAndName)
		return
	}

	if len(infos) > 1 {
		panic(infos)
	}

	clientset, err := o.ClientSet()
	if err != nil {
		return
	}

	ctx := context.TODO()
	opt := metav1.GetOptions{}
	info := infos[0]
	kind = strings.ToLower(info.Mapping.GroupVersionKind.Kind)
	name = info.Name
	switch info.Mapping.GroupVersionKind.Kind {
	case "Deployment":
		switch info.Mapping.GroupVersionKind.Group {
		case extensionsv1beta1.GroupName:
			deploy, failed := clientset.ExtensionsV1beta1().
				Deployments(info.Namespace).
				Get(ctx, info.Name, opt)
			if failed != nil {
				err = xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, failed)
				return
			}

			podTmpl = &deploy.Spec.Template.Spec
		case appsv1.GroupName:
			deploy, failed := clientset.AppsV1().Deployments(info.Namespace).Get(ctx, info.Name, opt)
			if failed != nil {
				err = xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, failed)
				return
			}

			podTmpl = &deploy.Spec.Template.Spec
		default:
			panic(info.Mapping.GroupVersionKind)
		}
	case "StatefulSet":
		if info.Mapping.GroupVersionKind.Group != appsv1.GroupName {
			panic(infos[0].Mapping.GroupVersionKind)
		}

		sfs, failed := clientset.AppsV1().StatefulSets(info.Namespace).Get(ctx, info.Name, opt)
		if failed != nil {
			err = xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, failed)
			return
		}

		podTmpl = &sfs.Spec.Template.Spec
	case "Job":
		if info.Mapping.GroupVersionKind.Group != batchv1.GroupName {
			panic(infos[0].Mapping.GroupVersionKind)
		}

		job, failed := clientset.BatchV1().Jobs(info.Namespace).Get(ctx, info.Name, opt)
		if failed != nil {
			err = xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, failed)
			return
		}

		podTmpl = &job.Spec.Template.Spec
	case "CronJob":
		if info.Mapping.GroupVersionKind.Group != batchv1.GroupName {
			panic(info.Mapping.GroupVersionKind)
		}

		job, failed := clientset.BatchV1beta1().CronJobs(info.Namespace).Get(ctx, info.Name, opt)
		if failed != nil {
			err = xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, err)
			return
		}

		podTmpl = &job.Spec.JobTemplate.Spec.Template.Spec
	case "DaemonSet":
		switch info.Mapping.GroupVersionKind.Group {
		case extensionsv1beta1.GroupName:
			ds, failed := clientset.ExtensionsV1beta1().DaemonSets(info.Namespace).Get(ctx, info.Name, opt)
			if failed != nil {
				err = xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, failed)
				return
			}

			podTmpl = &ds.Spec.Template.Spec
		case appsv1.GroupName:
			ds, failed := clientset.AppsV1().DaemonSets(info.Namespace).Get(ctx, info.Name, opt)
			if failed != nil {
				err = xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, failed)
				return
			}

			podTmpl = &ds.Spec.Template.Spec
		default:
			panic(info.Mapping.GroupVersionKind)
		}
	case "ReplicaSet":
		switch info.Mapping.GroupVersionKind.Group {
		case extensionsv1beta1.GroupName:
			rs, failed := clientset.ExtensionsV1beta1().ReplicaSets(info.Namespace).Get(ctx, info.Name, opt)
			if failed != nil {
				err = xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, failed)
				return
			}

			podTmpl = &rs.Spec.Template.Spec
		case appsv1.GroupName:
			rs, failed := clientset.AppsV1().ReplicaSets(info.Namespace).Get(ctx, info.Name, opt)
			if failed != nil {
				err = xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, failed)
				return
			}

			podTmpl = &rs.Spec.Template.Spec
		default:
			panic(info.Mapping.GroupVersionKind)
		}
	case "Pod":
		if info.Mapping.GroupVersionKind.Group != corev1.GroupName {
			panic(info.Mapping.GroupVersionKind)
		}

		pod, failed := clientset.CoreV1().Pods(info.Namespace).Get(ctx, info.Name, opt)
		if failed != nil {
			err = xerrors.Errorf("can't fetch %s/%s: %s", info.Mapping.GroupVersionKind, info.Name, failed)
			return
		}

		podTmpl = &pod.Spec
	default:
		err = xerrors.Errorf("object %s/%s is not supported", info.Mapping.GroupVersionKind, info.Name)
		return
	}

	return
}

func (o *DebugOptions) fetchDebugger(clientSet *kubernetes.Clientset) (debugger, container string, err error) {
	opts := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(o.labels).String(),
	}

	if len(o.debuggerPodName) > 0 {
		opts.FieldSelector = fields.Set{"metadata.name": o.debuggerPodName}.AsSelector().String()
	}

	debuggerList, err := clientSet.CoreV1().Pods(o.namespace).List(context.TODO(), opts)
	if err != nil {
		if !errors.IsNotFound(err) {
			err = nil
			return
		}

		err = xerrors.Errorf("can't fetch debugger: %s", err)
		return
	}

	if len(debuggerList.Items) == 0 {
		return
	}

	if len(debuggerList.Items) > 1 {
		err = xerrors.Errorf("found multiple debugger Pods. Try --debugger.")
		return
	}

	debugger = debuggerList.Items[0].Name
	if len(debuggerList.Items[0].Spec.Containers) == 1 {
		container = debuggerList.Items[0].Spec.Containers[0].Name
		if len(o.container) > 0 && container != o.container {
			err = xerrors.Errorf("container %s doesn't found in debugger %s. Try %s or remove -c",
				o.container, debugger)
		}

		return
	}

	if len(o.container) == 0 {
		var containers []string
		for _, c := range debuggerList.Items[0].Spec.Containers {
			containers = append(containers, c.Name)
		}

		err = xerrors.Errorf("debugger %s has more than 1 container. Specify one of %#v via -c",
			debuggerList, containers)
		return
	}

	for _, c := range debuggerList.Items[0].Spec.Containers {
		if c.Name == o.container {
			container = c.Name
		}
	}

	return
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

	clientset, err := o.ClientSet()
	if err != nil {
		return err
	}

	if !o.createNew {
		debugger, container, err := o.fetchDebugger(clientset)
		if err != nil {
			return err
		}

		if len(debugger) > 0 {
			fmt.Printf("Start bash of debugger Pod %s\n", debugger)
			return kubectl.Exec(debugger, o.namespace, container, "bash")
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

	if o.useHTTPProxy {
		proxyEnvs := []string{"http_proxy", "https_proxy", "no_proxy"}
		for _, env := range proxyEnvs {
			v, found := os.LookupEnv(env)
			if !found {
				v, found = os.LookupEnv(strings.ToUpper(env))
			}

			if !found {
				return xerrors.Errorf(`environment variable "%s" not found`, env)
			}

			if strings.HasPrefix(strings.ToLower(env), "http") {
				proxy, err := url.Parse(v)
				if err != nil {
					return xerrors.Errorf(`value of environment variable "%s", %s is not invalid: %s`,
						env, v, err)
				}

				if net.ParseIP(proxy.Host).IsLoopback() {
					return xerrors.Errorf(`proxy "%s" is a loopback URL. can't work in Pods.'`, env)
				}
			}

			container.Env = append(container.Env, corev1.EnvVar{
				Name:  env,
				Value: v,
			}, corev1.EnvVar{
				Name:  strings.ToUpper(env),
				Value: v,
			})
		}
	}

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

	debugger := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   o.instance,
			Labels: o.labels,
		},
		Spec: *o.podTmpl,
	}

	fmt.Printf("Create debugger Pod %s...\n", debugger.Name)
	newDebugger, err := clientset.CoreV1().Pods(o.namespace).Create(context.TODO(), debugger, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	fmt.Println("waiting Pod ready")
	watcher, err := clientset.CoreV1().Pods(o.namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: fields.Set{"metadata.name": newDebugger.Name}.AsSelector().String(),
		Watch:         true,
	})

	if err != nil {
		return err
	}

	err = utils.WaitUntilErrorOr(watcher, func(object runtime.Object) (b bool, err error) {
		fmt.Printf("Debugger %s updated\n", newDebugger.Name)
		return object.(*corev1.Pod).Status.Phase == corev1.PodRunning, nil
	})

	if err != nil {
		return xerrors.Errorf("can't start debugger Pod %s: %s", newDebugger.Name, err)
	}

	fmt.Printf("Debugger Pod %s is ready\n", newDebugger.Name)

	if !o.keepDebugger {
		fmt.Println("will be destroyed after session closed.")
		defer func() {
			fmt.Printf("Destroy debugger %s/%s...\n", newDebugger.Namespace, newDebugger.Name)
			if err = kubectl.Delete("Pod", newDebugger.Name, newDebugger.Namespace); err != nil {
				fmt.Fprintf(os.Stderr, "can't delete the debugger debugger: %s\n", err)
			}
		}()
	}

	fmt.Printf("Start bash of debugger Pod %s\n", newDebugger.Name)
	return kubectl.Exec(newDebugger.Name, newDebugger.Namespace, container.Name, "bash")
}

func NewCmdDebug(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDebugOptions(streams)

	var cmd = &cobra.Command{
		Use:   "debug",
		Short: "Debug running or failed workloads or images.",
		Long: `The image of the target workload will be mounted to a new Pod. You will see all original configurations 
even the filesystem in the new Pod, except the same entrypoint. Workloads could be Deployment, StatefulSet, DaemonSet,
ReplicaSet, Job, CronJob, and Pod.
`,
		Example: `# Debug a running or failed workload. And, install required drivers. This 
kubectl dev debug deploy foo --also-apply-csi-driver

# Debug a running or failed workload. Run the same command again could open a new session to the same debugger.
kubectl dev debug deploy foo

# If there are more than one debugger via same settings, Specify the Pod name to connect to one of them.
kubectl dev debug deploy foo --debugger=bar

# Force to start a new debugger.
kubectl dev debug sfs foo --create-new

# Specify container name if more than one containers in the Pod.
kubectl dev debug ds foo -c bar

# Debug a Pod with a new versioned image. 
kubectl dev debug pod foo --image bar:new-version

#Debug an image.
kubectl dev debug --image foo:latest

# The debugger Pod would be terminated after ONE of session closed except enabling --keep-debugger.
kubectl dev debug job foo --keep-debugger

# Use local network proxies.
kubectl dev debug cronjob foo --use-proxy
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
		"If set, the target cluster is assumed to be a minikube cluster.")
	cmd.Flags().StringVar(
		&o.debugBaseImage, "base", o.debugBaseImage,
		`Base image used to mount the target image. Command "bash" and "sleep" are required in the base image`)
	cmd.Flags().StringVarP(
		&o.container, "container", "c", o.container,
		"Container of the specified object if in which there are multiple containers")
	cmd.Flags().BoolVar(&o.installCSIDriver, "also-apply-csi-driver", false,
		`Apply the CSI driver "https://github.com/warm-metal/csi-driver-image". The driver is required. If you already have it installed, turn it off.`)
	cmd.Flags().BoolVar(&o.createNew, "create-new", false,
		"If set, always creates a new debugger Pod")
	cmd.Flags().StringVar(&o.image, "image", "",
		"The target image. If not set, use the image which the object used.")
	cmd.Flags().StringVar(&o.debuggerPodName, "debugger", "",
		"Debugger Pod name. If set along with --create-new=false, will find debugger has the specified name.")
	cmd.Flags().BoolVar(&o.keepDebugger, "keep-debugger", false,
		"If set, the debugger Pod won't be destroyed after the session closed.")
	cmd.Flags().BoolVar(&o.useHTTPProxy, "use-proxy", false, "If set, use current proxy settings.")
	o.AddFlags(cmd.Flags())

	return cmd
}
