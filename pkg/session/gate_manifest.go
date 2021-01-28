package session

import (
	"fmt"
	"github.com/warm-metal/kubectl-dev/pkg/dev"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const shellContextSyncImage = "docker.io/warmmetal/f2cm:v0.1.0"
const shellContextSidecar = "shell-context-sync"
const appContainer = "workspace"

func genAppManifest(name, appNs, image, coreNs string) *corev1.Pod {
	// FIXME handle volume mount, especially hostpath
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
					Name:         appContainer,
					Image:        image,
					Stdin:        true,
					VolumeMounts: []corev1.VolumeMount{sharedCtx},
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
			Volumes: []corev1.Volume{
				{
					Name: "shell-context",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}
}
