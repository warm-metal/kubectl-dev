package opts

import "github.com/warm-metal/kubectl-dev/pkg/kubectl"

type GlobalOptions struct {
	DevNamespace string
	kubectl.ConfigFlags
}
