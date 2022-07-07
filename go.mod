module github.com/warm-metal/kubectl-dev

go 1.15

require (
	github.com/docker/cli v20.10.13+incompatible
	github.com/docker/docker v20.10.7+incompatible
	github.com/fvbommel/sortorder v1.0.2 // indirect
	github.com/moby/buildkit v0.10.3
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.4.0
	github.com/spf13/pflag v1.0.5
	github.com/theupdateframework/notary v0.7.0
	github.com/tonistiigi/fsutil v0.0.0-20220506171851-e77355bad25d // indirect
	github.com/warm-metal/cliapp v0.0.0-20210508072337-996296ea0bf6
	k8s.io/api v0.24.2
	k8s.io/apimachinery v0.24.2
	k8s.io/cli-runtime v0.24.2
	k8s.io/client-go v0.24.2
	sigs.k8s.io/controller-runtime v0.8.2 // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/docker/docker => github.com/docker/docker v20.10.3-0.20220414164044-61404de7df1a+incompatible

replace github.com/docker/cli => github.com/docker/cli v20.10.3-0.20220603174727-3e9117b7e241+incompatible
