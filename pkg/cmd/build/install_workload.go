package build

import (
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const builderWorkloadName = "buildkitd"

func (o *BuilderInstallOptions) genBuildkitdWorkload() (*corev1.Service, *appsv1.Deployment, error) {
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

	//socket := corev1.HostPathSocket
	dir := corev1.HostPathDirectory
	dirOrCreate := corev1.HostPathDirectoryOrCreate
	bidirectional := corev1.MountPropagationBidirectional
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
							Name: "containerd-root",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: o.containerdRoot,
									Type: &dir,
								},
							},
						},
						{
							Name: "containerd-runtime",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: o.ContainerdRuntimeRoot,
									Type: &dir,
								},
							},
						},
						{
							Name: "buildkit-root",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: o.BuildkitRoot,
									Type: &dirOrCreate,
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
											Key:  "buildkitd.toml",
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
							Image: "docker.io/warmmetal/buildkit:0.8.1-1",
							Ports: []corev1.ContainerPort{
								{
									Name:          "service",
									ContainerPort: int32(o.Port),
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "containerd-root",
									MountPath: o.containerdRoot,
								},
								{
									Name:             "containerd-runtime",
									MountPath:        o.ContainerdRuntimeRoot,
									MountPropagation: &bidirectional,
								},
								{
									Name:             "buildkit-root",
									MountPath:        o.BuildkitRoot,
									MountPropagation: &bidirectional,
								},
								{
									Name:      "buildkitd-conf",
									MountPath: "/etc/buildkit/buildkitd.toml",
									SubPath:   "buildkitd.toml",
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

	if o.useHTTPProxy {
		proxies, err := utils.GetSysProxy()
		if err != nil {
			return nil, nil, err
		} else {
			deploy.Spec.Template.Spec.Containers[0].Env = append(
				deploy.Spec.Template.Spec.Containers[0].Env, proxies...)
		}
	}

	return svc, deploy, nil
}
