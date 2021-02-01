package session

import (
	"fmt"
	"github.com/warm-metal/kubectl-dev/pkg/dev"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	shellContextSyncImage = "docker.io/warmmetal/f2cm:v0.1.0"
	appContextImage       = "docker.io/warmmetal/app-context:v0.1.0"
	shellContextSidecar   = "shell-context-sync"
	appContainer          = "workspace"
	appRoot               = "/app-root"
	appImageVolume        = "app"
	csiDriverName         = "csi-image.warm-metal.tech"
)

func genAppManifest(name, appNs, image, coreNs string, hostPaths []string) *corev1.Pod {
	var hostVolumes []corev1.Volume
	var hostMounts []corev1.VolumeMount

	//propagation := corev1.MountPropagationBidirectional
	for i, path := range hostPaths {
		volume := fmt.Sprintf("hostpath-%d", i)
		hostVolumes = append(hostVolumes, corev1.Volume{
			Name: volume,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: path,
				},
			},
		})
		hostMounts = append(hostMounts, corev1.VolumeMount{
			Name:      volume,
			MountPath: path,
			//MountPropagation: &propagation,
		})
	}

	// FIXME HTTP proxy configuration
	sharedCtx := corev1.VolumeMount{
		Name:      "shell-context",
		MountPath: "/root",
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: appNs,
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:         "shell-context-initializer",
					Image:        shellContextSyncImage,
					Args:         []string{fmt.Sprintf("%s/%s=>/root", coreNs, dev.HistoryConfigMap)},
					VolumeMounts: []corev1.VolumeMount{sharedCtx},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  appContainer,
					Image: appContextImage,
					Env: []corev1.EnvVar{
						{
							Name:  "APP_ROOT",
							Value: appRoot,
						},
					},
					Stdin: true,
					VolumeMounts: append(
						hostMounts,
						sharedCtx,
						corev1.VolumeMount{
							Name:      appImageVolume,
							MountPath: appRoot,
						},
					),
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"SYS_ADMIN"},
						},
					},
				},
				{
					Name:  shellContextSidecar,
					Image: shellContextSyncImage,
					Args: []string{
						"-w",
						fmt.Sprintf("/root=>%s/%s", coreNs, dev.HistoryConfigMap),
					},
					VolumeMounts: []corev1.VolumeMount{sharedCtx},
				},
			},
			Volumes: append(hostVolumes,
				corev1.Volume{
					Name: appImageVolume,
					VolumeSource: corev1.VolumeSource{
						CSI: &corev1.CSIVolumeSource{
							Driver: csiDriverName,
							VolumeAttributes: map[string]string{
								"image": image,
							},
						},
					},
				},
				corev1.Volume{
					Name: "shell-context",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			),
		},
	}
}
