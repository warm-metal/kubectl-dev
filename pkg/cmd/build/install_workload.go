package build

import (
	"context"
	"fmt"
	"golang.org/x/xerrors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const builderWorkloadName = "buildkitd"

func (o *BuilderInstallOptions) genBuildkitdWorkload() (*corev1.Service, *appsv1.Deployment) {
	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      builderWorkloadName,
			Namespace: o.namespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:        builderWorkloadName,
					Protocol:    corev1.ProtocolTCP,
					AppProtocol: nil,
					Port:        int32(o.Port),
					TargetPort:  intstr.FromInt(o.Port),
				},
			},
			Selector: map[string]string{
				"owner": "warm-metal.tech",
				"app":   "builder",
			},
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}

	socket := corev1.HostPathSocket
	dir := corev1.HostPathDirectory
	probe := &corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{Command: []string{
				"buildctl",
				"debug",
				"workers",
			}},
		},
		InitialDelaySeconds: 5,
		PeriodSeconds:       30,
	}

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      builderWorkloadName,
			Namespace: o.namespace,
			Labels:    svc.Spec.Selector,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &numReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: svc.Spec.Selector,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: svc.Spec.Selector,
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "snapshot-home",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: o.containerdRoot,
									Type: &dir,
								},
							},
						},
						{
							Name: "buildkitd-conf",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: buildkitdTomlConfigMap,
									},
									Items: []corev1.KeyToPath{
										{
											Key: "buildkitd.toml",
											Path: "buildkitd.toml",
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  builderWorkloadName,
							Image: "moby/buildkit:master",
							Ports: []corev1.ContainerPort{
								{
									Name:          "service",
									ContainerPort: int32(o.Port),
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "snapshot-home",
									MountPath: o.containerdRoot,
								},
								{
									Name:      "buildkitd-conf",
									MountPath: "/etc/buildkit/buildkitd.toml",
									SubPath: "buildkitd.toml",
								},
							},
							LivenessProbe:  probe,
							ReadinessProbe: probe,
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
						},
					},
				},
			},
		},
	}

	if len(o.ContainerdSocketPath) > 0 {
		deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: "containerd-socket",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: o.ContainerdSocketPath,
						Type: &socket,
					},
				},
			},
		)
		deploy.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			deploy.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "containerd-socket",
				MountPath: o.ContainerdSocketPath,
			},
		)
	}

	return svc, deploy
}

func fetchBuilderEndpoints(clientset *kubernetes.Clientset) (buildkitAddrs []string, err error) {
	svc, err := clientset.CoreV1().Services(builderNamespace).
		Get(context.TODO(), builderWorkloadName, metav1.GetOptions{})
	if err != nil {
		return nil, xerrors.Errorf(
			`can't fetch builder endpoint from Service "%s/%s": %s`, builderNamespace, builderWorkloadName, err)
	}

	svcPort := int32(0)
	nodePort:= int32(0)
	for _, port := range svc.Spec.Ports {
		if port.Name != builderWorkloadName {
			continue
		}

		svcPort = port.Port
		nodePort = port.NodePort
	}

	if svcPort > 0 {
		for _, ingress := range svc.Status.LoadBalancer.Ingress {
			if len(ingress.Hostname) > 0 {
				buildkitAddrs = append(buildkitAddrs, fmt.Sprintf("tcp://%s:%d", ingress.Hostname, svcPort))
			}

			if len(ingress.IP) > 0 {
				buildkitAddrs = append(buildkitAddrs, fmt.Sprintf("tcp://%s:%d", ingress.IP, svcPort))
			}
		}
	}

	buildkitAddrs = append(buildkitAddrs, fmt.Sprintf("tcp://%s:%d", svc.Spec.ClusterIP, svcPort))
	if nodePort == 0 {
		return
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, xerrors.Errorf(`can't list node while enumerating Service NodePort: %s`, err)
	}

	for _, node := range nodes.Items {
		for _, addr := range node.Status.Addresses {
			if len(addr.Address) > 0 {
				buildkitAddrs = append(buildkitAddrs, fmt.Sprintf("tcp://%s:%d", addr.Address, nodePort))
			}
		}
	}

	return
}
