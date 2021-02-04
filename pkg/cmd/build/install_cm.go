package build

import (
	"bytes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"text/template"
)

const buildkitdToml = `debug = true
# root is where all buildkit state is stored.
root = "{{.BuildkitRoot}}"
snapshot-root = "{{.SnapshotRoot}}"
# insecure-entitlements allows insecure entitlements, disabled by default.
insecure-entitlements = [ "network.host", "security.insecure" ]

[grpc]
  address = [ "unix:///run/buildkit/buildkitd.sock", "tcp://0.0.0.0:{{.Port}}" ]
  uid = 0
  gid = 0

[worker.oci]
  enabled = false

[worker.containerd]
  address = "{{.ContainerdRuntimeRoot}}/containerd.sock"
  enabled = true
  platforms = [ "linux/amd64", "linux/arm64" ]
  namespace = "k8s.io"
  gc = true
  [[worker.containerd.gcpolicy]]
    keepBytes = 10240000000
	keepDuration = 3600
`

const buildkitdTomlConfigMap = "buildkitd.toml"

func (o BuilderInstallOptions) genBuildkitdToml() *corev1.ConfigMap {
	configTmpl, err := template.New("buildkitd.toml").Parse(buildkitdToml)
	if err != nil {
		panic(err)
	}

	buf := &bytes.Buffer{}
	err = configTmpl.Execute(buf, &o)
	if err != nil {
		panic(err)
	}

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildkitdTomlConfigMap,
			Namespace: o.namespace,
		},
		Immutable: &readOnly,
		Data: map[string]string{
			"buildkitd.toml": buf.String(),
		},
	}
}
