# kubectl-dev

`kubectl-dev` is a kubectl plugin to support **image building on container runtime other than Docker**,
**workload and image debugging in k8s clusters**, and `cliapp` in single-node clusters.

Currently, the plugin can only work on **containerd**. All features work well in minikube clusters.

## Features
* Debug workloads or images,
* Image Builder for containerd, w/ host HTTP_PROXY settings,
* [CliApp](https://github.com/warm-metal/cliapp#cliapp).

## Install

### From Homebrew
The Homebrew formulae is available for MacOS.

```shell script
brew install warm-metal/rc/kubectl-dev
```

### From the pre-built binary
You can also download the pre-built binary.

```shell script
# For MacOS, the administrator privilege is required to save kubectl-dev to /usr/local/bin. Run
sudo sh -c 'curl -skL https://github.com/warm-metal/kubectl-dev/releases/download/v0.1.1/kubectl-dev.darwin-amd64.tar.xz | tar -C /usr/local/bin/ -xpf -'

# For Linux, run
sudo sh -c 'curl -skL https://github.com/warm-metal/kubectl-dev/releases/download/v0.1.1/kubectl-dev.linux-amd64.xz | tar -C /usr/local/bin/ -xpf -'
```

## Initialization
After installed, run one of the commands below to install deps.
```shell script
kubectl dev prepare

# For minikube clusters
kubectl dev prepare --minikube

# Inherit current HTTP_PROXY in the buildkit workspace.
# If you are in mainland China, this flag could accelerate the speed of image and dependency pulling while building.
kubectl dev prepare --minikube --use-proxy

# Install cliapp and set the environment variable to buildkit.
kubectl dev prepare --builder-env GOPROXY=goproxy.cn
```

## Usage
### Build image or binary

The `kubectl-dev build` command is full compatible to the `docker build` command.

```shell script
# Build image foo:bar using Dockerfile in the current directory.
kubectl dev build -t foo:bar

# Build image foo:bar using foobar.dockerfile as the Dockerfile in diretory ~/image.
kubectl dev build -t foo:bar -f foobar.dockerfile ~/image
```

The build command also can copy artifacts of a stage to local directory which is one of features buildkit supported.

```shell script
# Build the stage mac-cli and copy generated to the local directory _output.
kubectl dev build  -f hack/dev/Dockerfile --local _output/ --target mac-cli
```

### Debug workloads

If an app failed, it would crash, wait for deps and has no responding, fails on some libraries, 
say some .so files, or get wrong mounted ConfigMaps or Secrets.
K8s provides nothing to figure them out. The only thing may help is logs your app printed.

The `debug` command provides a new way to start the workload. It creates an identical Pod in the same namespace,
except the image of the target container. `debug` opens a bash session after the Pod started. 
The target image is mount to `/app-root`. 
You can check the original image context or debug the binary in the opened session.

Deployment, StatefulSet, DaemonSet, ReplicaSet, Job, CronJob, and Pod are all supported. 

```bash
# Debug a running or failed workload. Run the same command again could open a new session to the same debugger.
kubectl dev debug deploy foo

# The debugger Pod would not fork environment variables from the original workload.
kubectl dev debug deploy foo --with-original-envs

# Specify container name if more than one containers in the Pod.
kubectl dev debug ds foo -c bar

# Debug a Pod with a new versioned image. 
kubectl dev debug pod foo --image bar:new-version

#Debug an image.
kubectl dev debug --image foo:latest

# Pass the local HTTP_PROXY to the debugger Pod.
kubectl dev debug cronjob foo --use-proxy
```

### Use cliapps

```bash

```

## Build
```shell script
# For MacOS, run
kubectl dev build  -f hack/dev/Dockerfile --local _output/ --target mac-cli

# For Linux, run
kubectl dev build  -f hack/dev/Dockerfile --local _output/ --target linux-cli
```

## Prepare a minikube cluster for program development

It offers you some capabilities to build images and debug them in k8s clusters directly.
You don't need to install many runtime and many more versions of them in your laptop.
No runtime changing and management. Also, no out-of-date garbage deps. All these are replaced by a k8s cluster.