# kubectl-dev

**kubectl-dev** is a kubectl plugin to support development activities on k8s.

We want to move all your development activities, image building, debugging, as well as deps resolving,
all of them into k8s clusters. You don't need to install many runtime and many more versions of them in your laptop.
No runtime changing and management. Also, no out-of-date garbage deps. All these are replaced by a k8s cluster.

Currently, the plugin can only work with **containerd**. All features work well in a minikube cluster.

## Install

If you have golang toolchains, run
```go
go install github.com/warm-metal/kubectl-dev
```

## Usage

```bash
# Debug the Deployment named workload and install the CSI driver.
kubectl dev debug deploy workload --also-apply-csi-driver

# Install build toolchains.
kubectl dev build install

# Install build toolchains in minikube cluster.
kubectl dev build install --minikube

# Build image in cluster using docker parameters and options.
kubectl dev build -t docker.io/warmmetal/image:tag -f test.dockerfile .
```
