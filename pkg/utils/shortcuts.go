package utils

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func FetchServiceEndpoints(ctx context.Context, clientset *kubernetes.Clientset, namespace, service, port string) (addrs []string, err error) {
	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, service, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf(
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
		nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf(`can't list node while enumerating Service NodePort: %s`, err)
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
