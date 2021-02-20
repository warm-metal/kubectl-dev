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
	"context"
	"github.com/spf13/pflag"
	"github.com/warm-metal/kubectl-dev/pkg/cmd"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/util/exec"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	flags := pflag.NewFlagSet("kubectl-dev", pflag.ExitOnError)
	pflag.CommandLine = flags

	signCh := make(chan os.Signal, 3)
	defer close(signCh)
	signal.Ignore(syscall.SIGPIPE)
	signal.Notify(signCh, os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	errCh := make(chan error)
	go func() {
		defer close(errCh)
		root := cmd.NewCmdDev(genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
		errCh <- root.ExecuteContext(ctx)
	}()

	for {
		select {
		case err, ok := <-errCh:
			if !ok {
				break
			}

			signal.Stop(signCh)

			if exit, ok := err.(exec.CodeExitError); ok {
				os.Exit(exit.Code)
			}

			os.Exit(1)
			return

		case <-signCh:
			signal.Stop(signCh)
			cancel()
		}
	}
}
