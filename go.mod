module github.com/warm-metal/kubectl-dev

go 1.15

require github.com/spf13/cobra v1.1.1

require (
	github.com/jaguilar/vt100 v0.0.0-20201024211400-81de19cb81a4 // indirect
	github.com/moby/buildkit v0.8.1
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.20.1
	k8s.io/apimachinery v0.20.1
	k8s.io/cli-runtime v0.20.1
	k8s.io/client-go v0.20.1
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305
