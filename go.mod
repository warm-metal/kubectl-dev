module github.com/warm-metal/kubectl-dev

go 1.15

require github.com/spf13/cobra v1.1.1

require (
	cloud.google.com/go v0.76.0 // indirect
	github.com/Azure/go-autorest/autorest v0.11.18 // indirect
	github.com/go-logr/zapr v0.4.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/googleapis/gnostic v0.5.4 // indirect
	github.com/gophercloud/gophercloud v0.15.0 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/jaguilar/vt100 v0.0.0-20201024211400-81de19cb81a4 // indirect
	github.com/moby/buildkit v0.8.1
	github.com/moby/term v0.0.0-20200915141129-7f0af18e79f2 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.9.0 // indirect
	github.com/prometheus/procfs v0.6.0 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/warm-metal/cliapp v0.0.0-20210219041214-f93de923a220
	github.com/warm-metal/cliapp-session-gate v0.0.0-20210222170540-ffea51093c70
	go.opencensus.io v0.22.5 // indirect
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0 // indirect
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad // indirect
	golang.org/x/net v0.0.0-20210119194325-5f4716e94777 // indirect
	golang.org/x/oauth2 v0.0.0-20210210192628-66670185b0cd // indirect
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c // indirect
	golang.org/x/term v0.0.0-20201210144234-2321bbc49cbf // indirect
	golang.org/x/text v0.3.5 // indirect
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20210202153253-cf70463f6119 // indirect
	google.golang.org/grpc v1.35.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.20.2
	k8s.io/apiextensions-apiserver v0.20.2 // indirect
	k8s.io/apimachinery v0.20.2
	k8s.io/cli-runtime v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/klog v1.0.0 // indirect
	k8s.io/klog/v2 v2.5.0 // indirect
	k8s.io/kube-openapi v0.0.0-20210211043216-66d8d84e87dd // indirect
	sigs.k8s.io/controller-runtime v0.8.2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.0.3 // indirect
	sigs.k8s.io/yaml v1.2.0
)

replace github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305
