package utils

import (
	"context"
	"fmt"
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
