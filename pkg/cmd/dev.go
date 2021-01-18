/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"github.com/spf13/cobra"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/build"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewCmdDev(streams genericclioptions.IOStreams) *cobra.Command {
	var devCmd = &cobra.Command{
		Use:   "kubectl-dev",
		Short: "kubectl plugin to support development in k8s clusters",
		Example: `# Debug the Deployment named workload and install the CSI driver.
kubectl dev debug deploy foo --also-apply-csi-driver

# Debug an image. 
kubectl dev debug --image foo:latest

# Install build toolchains.
kubectl dev build install

# Install build toolchains in minikube cluster.
kubectl dev build install --minikube

# Build image in cluster using docker parameters and options.
kubectl dev build -t foo:tag -f Dockerfile .`,
	}

	devCmd.AddCommand(NewCmdDebug(streams))
	devCmd.AddCommand(build.NewCmd(streams))
	return devCmd
}
