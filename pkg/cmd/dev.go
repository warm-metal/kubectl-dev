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
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewCmdDev(streams genericclioptions.IOStreams) *cobra.Command {
	var devCmd = &cobra.Command{
		Use:   "kubectl-dev",
		Short: "kubectl plugin to support development on k8s",
		Long: `Debug workloads or images. For example:

kubectl dev debug po failed-po-name

kubectl dev debug deploy deploy-name

kubectl dev debug docker.io/warmmetal/image:label`,
	}

	devCmd.AddCommand(NewDebugDev(streams))

	return devCmd
}
