# kubectl-dev

**kubectl-dev** is a kubectl plugin to support development activities on k8s.

The `debug` command can debug running or failed workloads, or images.
It mounts the target image to a new Pod as well as workload configurations then open a new session of the Pod.
This feature depends on `warm-metal/csi-driver-image`. It would be installed in the cluster.

## Install

If you have golang toolchains, run
```go
go install github.com/warm-metal/kubectl-dev
```

## Usage

```bash
# Debug the Deployment named workload
kubectl dev debug deploy workload

# Install build toolchains
kubectl dev build install

# Build an image
kubectl dev build -t docker.io/warmmetal/image:tag -f test.dockerfile .
```
