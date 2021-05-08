module github.com/warm-metal/kubectl-dev

go 1.15

require github.com/spf13/cobra v1.1.1

require (
	github.com/docker/cli v20.10.6+incompatible
	github.com/docker/docker v20.10.6+incompatible
	github.com/fvbommel/sortorder v1.0.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/googleapis/gnostic v0.5.4 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jaguilar/vt100 v0.0.0-20201024211400-81de19cb81a4 // indirect
	github.com/moby/buildkit v0.8.3
	github.com/pkg/errors v0.9.1
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/theupdateframework/notary v0.7.0
	github.com/warm-metal/cliapp v0.0.0-20210508072337-996296ea0bf6
	go.opencensus.io v0.22.5 // indirect
	golang.org/x/oauth2 v0.0.0-20210210192628-66670185b0cd // indirect
	golang.org/x/text v0.3.5 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20210202153253-cf70463f6119 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.21.0
	k8s.io/apimachinery v0.21.0
	k8s.io/cli-runtime v0.20.2
	k8s.io/client-go v0.21.0
	sigs.k8s.io/controller-runtime v0.8.2 // indirect
	sigs.k8s.io/kustomize v2.0.3+incompatible // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305
