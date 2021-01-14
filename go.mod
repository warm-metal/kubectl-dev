module github.com/warm-metal/kubectl-dev

go 1.15

require github.com/spf13/cobra v1.1.1

require (
	github.com/spf13/pflag v1.0.5
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c
	k8s.io/api v0.20.1
	k8s.io/apimachinery v0.20.1
	k8s.io/cli-runtime v0.20.1
	k8s.io/client-go v0.20.1
	sigs.k8s.io/yaml v1.2.0
)
