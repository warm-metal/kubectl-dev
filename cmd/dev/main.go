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
package main

import (
	"github.com/spf13/pflag"
	"github.com/warm-metal/kubectl-dev/pkg/cmd"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/util/exec"
	"os"
)

func main() {
	flags := pflag.NewFlagSet("kubectl-dev", pflag.ExitOnError)
	pflag.CommandLine = flags

	root := cmd.NewCmdDev(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	if err := root.Execute(); err != nil {
		if exit, ok := err.(exec.CodeExitError); ok {
			os.Exit(exit.Code)
		}

		os.Exit(1)
	}
}
