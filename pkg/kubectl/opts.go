package kubectl

import (
	"github.com/spf13/pflag"
	"golang.org/x/xerrors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

type ConfigFlags struct {
	configFlags *genericclioptions.ConfigFlags
}

func NewConfigFlags() ConfigFlags {
	return ConfigFlags{
		configFlags: genericclioptions.NewConfigFlags(true),
	}
}

func (o ConfigFlags) ClientSet() (*kubernetes.Clientset, error) {
	config, err := o.configFlags.ToRESTConfig()
	if err != nil {
		return nil, xerrors.Errorf("invalid kubectl configuration: %s", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, xerrors.Errorf("invalid kubectl configuration: %s", err)
	}

	return clientset, err
}

func (o *ConfigFlags) AddPersistentFlags(flags *pflag.FlagSet) {
	o.configFlags.AddFlags(flags)
}

func (o ConfigFlags) Raw() *genericclioptions.ConfigFlags {
	return o.configFlags
}
