# kubectl-dev

**kubectl-dev** is a kubectl plugin to support development activities on k8s.

The `debug` command can debug running or failed workloads, or images.
It mounts the target image to a new Pod as well as workload configurations then open a new session of the Pod.
This feature depends on `warm-metal/csi-driver-image`. It would be installed in the cluster.
