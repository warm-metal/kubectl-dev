package utils

import (
	"context"
	"fmt"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func EnsureNamespace(clientset *kubernetes.Clientset, namespace string) (err error) {
	c := clientset.CoreV1().Namespaces()
	_, err = c.Get(context.TODO(), namespace, metav1.GetOptions{})
	if err == nil {
		return
	}

	if !errors.IsNotFound(err) {
		return err
	}

	fmt.Println("Create namespace", namespace, "...")
	_, err = c.Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})

	return
}

func EnsureCM(clientset *kubernetes.Clientset, cm *corev1.ConfigMap) (err error) {
	c := clientset.CoreV1().ConfigMaps(cm.Namespace)
	_, err = c.Get(context.TODO(), cm.Name, metav1.GetOptions{})
	if err == nil {
		return
	}

	if !errors.IsNotFound(err) {
		return err
	}

	fmt.Println("Create ConfigMap", cm.Namespace, "/", cm.Name, "...")
	_, err = c.Create(context.TODO(), cm, metav1.CreateOptions{})
	return
}

func EnsureClusterRole(clientset *kubernetes.Clientset, role *rbacv1.ClusterRole) (err error) {
	c := clientset.RbacV1().ClusterRoles()
	_, err = c.Get(context.TODO(), role.Name, metav1.GetOptions{})
	if err == nil {
		return
	}

	if !errors.IsNotFound(err) {
		return err
	}

	fmt.Println("Create ClusterRole", role.Name, "...")
	_, err = c.Create(context.TODO(), role, metav1.CreateOptions{})
	return
}

func EnsureClusterRoleBinding(clientset *kubernetes.Clientset, binding *rbacv1.ClusterRoleBinding) (err error) {
	c := clientset.RbacV1().ClusterRoleBindings()
	_, err = c.Get(context.TODO(), binding.Name, metav1.GetOptions{})
	if err == nil {
		return
	}

	if !errors.IsNotFound(err) {
		return err
	}
	fmt.Println("Create ClusterRoleBinding", binding.Name, "...")
	_, err = c.Create(context.TODO(), binding, metav1.CreateOptions{})
	return
}

func FetchServiceEndpoints(clientset *kubernetes.Clientset, namespace, service, port string) (addrs []string, err error) {
	svc, err := clientset.CoreV1().Services(namespace).
		Get(context.TODO(), service, metav1.GetOptions{})
	if err != nil {
		return nil, xerrors.Errorf(
			`can't fetch endpoint from Service "%s/%s": %s`, namespace, service, err)
	}

	svcPort := int32(0)
	nodePort := int32(0)
	for _, p := range svc.Spec.Ports {
		if p.Name != port {
			continue
		}

		svcPort = p.Port
		nodePort = p.NodePort
	}

	if svcPort > 0 {
		for _, ingress := range svc.Status.LoadBalancer.Ingress {
			if len(ingress.Hostname) > 0 {
				// FIXME deduct scheme from port protocol
				addrs = append(addrs, fmt.Sprintf("tcp://%s:%d", ingress.Hostname, svcPort))
			}

			if len(ingress.IP) > 0 {
				addrs = append(addrs, fmt.Sprintf("tcp://%s:%d", ingress.IP, svcPort))
			}
		}
	}

	if nodePort > 0 {
		nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, xerrors.Errorf(`can't list node while enumerating Service NodePort: %s`, err)
		}

		for _, node := range nodes.Items {
			for _, addr := range node.Status.Addresses {
				if len(addr.Address) > 0 {
					addrs = append(addrs, fmt.Sprintf("tcp://%s:%d", addr.Address, nodePort))
				}
			}
		}
	}

	addrs = append(addrs, fmt.Sprintf("tcp://%s:%d", svc.Spec.ClusterIP, svcPort))
	return
}
