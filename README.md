# kubectl-dev

**kubectl-dev** is a kubectl plugin to support development activities on k8s.

It offers you some capabilities to build images and debug them in k8s clusters directly.
You don't need to install many runtime and many more versions of them in your laptop.
No runtime changing and management. Also, no out-of-date garbage deps. All these are replaced by a k8s cluster.

Currently, the plugin can only work with **containerd**. All features work well in a minikube cluster.

### How to debug failed apps efficiently?

If a app failed, it could crash, wait for deps and has no responding, fails on some libraries, 
say some .so files, or get wrong mounted ConfigMaps or Secrets.
K8s provides nothing to figure them out. The only thing may help is logs your app printed.

The `debug` command provides a new way to start the workload. It creates an identical Pod in the same namespace,
except the image of the target container. `debug` opens a bash session after the Pod started. 
The target image is mount to `/image`. Its original parameters are set in the environment variable `IMAGE_ARGS`.
You can check the original image context or debug the binary in the opened session.

 This command also supports Docker as container runtime. But, it needs a few more steps to install deps.
See [Install csi-driver-image on Docker](https://github.com/warm-metal/csi-driver-image#docker).

### We are trying to install client-side apps in the cluster.

## Install

The Homebrew formulae is available for MacOS.

```shell script
brew install warm-metal/rc/kubectl-dev
```

You can also download the pre-build binary.

```shell script
# For MacOS, the administrator privilege is required to save kubectl-dev to /usr/local/bin. Run
sudo sh -c 'curl -skL https://github.com/warm-metal/kubectl-dev/releases/download/v0.1.1/kubectl-dev.darwin-amd64.tar.xz | tar -C /usr/local/bin/ -xpf -'

# For Linux, run
sudo sh -c 'curl -skL https://github.com/warm-metal/kubectl-dev/releases/download/v0.1.1/kubectl-dev.linux-amd64.xz | tar -C /usr/local/bin/ -xpf -'
```

`kubectl` is required to manage necessary objects. We also assumed that you have a k8s cluster, or a minikube cluster.

The [csi-driver-image](https://github.com/warm-metal/csi-driver-image) is also needed by the `debug` command.
You can install the predefined manifests via the `--also-apply-csi-driver` option while starting the debug command.
Or, you can modify and apply manifests manually.  

## Usage

```bash
# Debug the Deployment named workload and install the CSI driver.
 # For the containerd runtime,
 kubectl dev debug deploy workload --also-apply-csi-driver

 # For the containerd runtime in minikube clusters,
 kubectl dev debug deploy workload --also-apply-csi-driver --minikube

 # For the docker runtime,
 kubectl dev debug deploy workload --also-apply-csi-driver --docker

# Install build toolchains.
kubectl dev build install

# Install build toolchains in minikube cluster.
kubectl dev build install --minikube

# Build image in cluster using docker parameters and options.
kubectl dev build -t docker.io/warmmetal/image:tag -f test.dockerfile .
```

## Build
```shell script
# For MacOS, run
kubectl dev build  -f hack/dev/Dockerfile --local _output/ --target mac-cli

# For Linux, run
kubectl dev build  -f hack/dev/Dockerfile --local _output/ --target linux-cli
```
