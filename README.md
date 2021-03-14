# kubectl-dev

`kubectl-dev` is a kubectl plugin to support **image building**, **workload debugging**, 
and **CliApp** especially in a single-node minikube cluster which is used to replace Docker Desktop.

Currently, the plugin can only work on **containerd**.

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
# For minikube clusters
kubectl dev prepare --minikube

# Inherit current HTTP_PROXY in the buildkit workspace.
# If you are in mainland China, this flag could accelerate the speed of image and dependency pulling while building.
kubectl dev prepare --minikube --use-proxy

# Install cliapp and set the environment variable to buildkit.
kubectl dev prepare --builder-env GOPROXY='https://goproxy.cn|https://goproxy.io|direct'

# For containerd
kubectl dev prepare
```

## Usage
### Build image or binary

The `kubectl-dev build` command is fully compatible to the `docker build` command.
`no-cache`, `build-arg` and `target` are also supported.

```shell script
# Build image foo:bar using Dockerfile in the current directory.
kubectl dev build -t foo:bar

# Build image foo:bar using foobar.dockerfile as the Dockerfile in diretory ~/image.
kubectl dev build -t foo:bar -f foobar.dockerfile ~/image
```

The build command also can copy artifacts from a stage to a local directory.

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

```shell script
# Debug a running or failed workload. Run the same command again could open a new session to the same debugger.
kubectl dev debug -n cliapp-system deploy buildkitd

# The debugger Pod would not fork environment variables from the original workload.
kubectl dev debug -n cliapp-system deploy buildkitd --with-original-envs=false

# Specify container name if more than one containers in the Pod. Or, an error would arise.
kubectl dev debug -n cliapp-system deploy buildkitd -c buildkitd

# Pass the local HTTP_PROXY to the debugger Pod.
kubectl dev debug -n cliapp-system deploy buildkitd --use-proxy

# Debug a Pod with a new versioned image. 
kubectl dev debug pod foo --image bar:new-version

#Debug an image.
kubectl dev debug --image foo:latest
```

The default distro of debugger is `alpine`. `ubuntu` would be another option
You can also choose one of `bash` or `zsh` as your favorite in debuggers via option `--shell`.
```shell script
kubectl dev debug -n cliapp-system deploy buildkitd --with-original-envs=false --shell zsh --distro ubuntu
```

### Use CliApp

CliApp provides the capability of running cli commands, which are installed in the cluster, from a local terminal.

Besides installing a CliApp object in the cluster, a shortcut w/ the same is created in the directory **~/.cliapps/**
and is linked to **/usr/local/bin/**.

```shell script
# Install cliapp crictl via image docker.io/warmmetal/app-crictl:v0.1.0.
# The last argument "crictl" shows that command crictl will be executed in the Pod once the app is executed. 
# If omitted, the command same with the app name is started instead.
sudo -E kubectl dev app install --name crictl \
	--image docker.io/warmmetal/app-crictl:v0.1.0 \
	--env CONTAINER_RUNTIME_ENDPOINT=unix:///var/run/containerd/containerd.sock \
	--hostpath /var/run/containerd/containerd.sock --use-proxy \
	crictl
# ❯ command -v crictl
# /usr/local/bin/crictl
# ❯ ls -l /usr/local/bin/crictl
# lrwxr-xr-x  1 root  wheel  25 Mar 14 18:57 /usr/local/bin/crictl -> /Users/kh/.cliapps/crictl
```

You can install a CliApp via a Dockerfile and the builtin buildkit will help build the necessary image.
```shell script
sudo -E kubectl dev app install --name ctr \
	--dockerfile https://raw.githubusercontent.com/warm-metal/cliapps/master/ctr/Dockerfile \
	--env CONTAINERD_NAMESPACE=k8s.io \
	--hostpath /var/run/containerd/containerd.sock --use-proxy
```

## Build

```shell script
# For MacOS, run
kubectl dev build  -f hack/dev/Dockerfile --local _output/ --target mac-cli

# For Linux, run
kubectl dev build  -f hack/dev/Dockerfile --local _output/ --target linux-cli
```

Or, use Makefile instead.

## Prepare a minikube cluster for program development

`kubectl-dev` offers you some capabilities to build images and debug them in k8s clusters directly.
You don't need to install many runtime and many more versions of them in your laptop.
No runtime changing and management. Also, no out-of-date dep garbage. All these are replaced by a k8s cluster.

To own a local minikube cluster on your laptop is not easy as running `minikube start`. It could be a little tricky.

### Create (start the first time) a cluster w/ containerd

```shell script
mini_create() {
  PROFILE=minikube
  if [[ $# -gt 0 ]]; then
    PROFILE=$1
  fi

  minikube start -p $PROFILE \
      --service-cluster-ip-range="10.24.0.0/16" \
      --container-runtime=containerd \
      --memory=8g \
      --cpus=4 \
      --disk-size=100g
}
```

### Start an exited cluster

After the cluster created, we must disable both `--preload` and `--cache-images`. Or,  
with --preload enabled, the containerd content store will be override by a pre-downloaded tarball.
If --cache-images enabled, minikube always try to save images to local tarballs.

```shell script
mini_start() {
  PROFILE=minikube
  if [[ $# -gt 0 ]]; then
    PROFILE=$1
  fi

  minikube start -p $PROFILE --preload=false --cache-images=false
}
```

### Time Sync
With hyberkit, the guest can't sync its datetime to local host. The easiest way to keep time sync is using NTP.
But, if you are in a poor network or behind a powerful firewall, the default NTP settings is useless.

To fix it, you can try another wheel we built, [kube-systemd](https://github.com/warm-metal/kube-systemd).