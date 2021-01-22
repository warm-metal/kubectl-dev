package dev

import (
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"k8s.io/client-go/kubernetes"
)

func Prepare(clientset *kubernetes.Clientset, namespace string) error {
	if err := utils.EnsureNamespace(clientset, namespace); err != nil {
		return err
	}

	cm := genHistoryCM(namespace)
	if err := utils.EnsureCM(clientset, cm); err != nil {
		return err
	}

	cr := genClusterRole()
	if err := utils.EnsureClusterRole(clientset, cr); err != nil {
		return err
	}

	binding := genHistoryClusterRoleBinding()
	if err := utils.EnsureClusterRoleBinding(clientset, binding); err != nil {
		return err
	}

	return nil
}
