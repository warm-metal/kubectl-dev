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
	"context"
	"fmt"
	"github.com/docker/cli/cli/command"
	cliconfig "github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	configtypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/cli/cli/context/docker"
	ctxStore "github.com/docker/cli/cli/context/store"
	manifestStore "github.com/docker/cli/cli/manifest/store"
	"github.com/docker/cli/cli/registry/client"
	"github.com/docker/cli/cli/streams"
	"github.com/docker/cli/cli/trust"
	dockerTypes "github.com/docker/docker/api/types"
	registrytypes "github.com/docker/docker/api/types/registry"
	dockerCli "github.com/docker/docker/client"
	"github.com/docker/docker/registry"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	notaryCli "github.com/theupdateframework/notary/client"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/opts"
	"golang.org/x/xerrors"
	"io"
	"io/ioutil"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"os"
	"strings"
)

type CredOptions struct {
	*opts.GlobalOptions

	streams genericclioptions.IOStreams

	in  *streams.In
	out *streams.Out

	server        string
	username      string
	password      string
	passwordStdin bool

	isDefaultRegistry bool
}

func (o *CredOptions) NotaryClient(imgRefAndAuth trust.ImageRefAndAuth, actions []string) (notaryCli.Repository, error) {
	panic("not used")
}

func (o *CredOptions) Client() dockerCli.APIClient {
	panic("not used")
}

func (o *CredOptions) Out() *streams.Out {
	return o.out
}

func (o *CredOptions) Err() io.Writer {
	return o.streams.ErrOut
}

func (o *CredOptions) In() *streams.In {
	return o.in
}

func (o *CredOptions) SetIn(in *streams.In) {
	panic("not used")
}

func (o *CredOptions) Apply(ops ...command.DockerCliOption) error {
	panic("not used")
}

func (o *CredOptions) ConfigFile() *configfile.ConfigFile {
	return cliconfig.LoadDefaultConfigFile(o.streams.ErrOut)
}

func (o *CredOptions) ServerInfo() command.ServerInfo {
	panic("not used")
}

func (o *CredOptions) ClientInfo() command.ClientInfo {
	panic("not used")
}

func (o *CredOptions) DefaultVersion() string {
	panic("not used")
}

func (o *CredOptions) ManifestStore() manifestStore.Store {
	panic("not used")
}

func (o *CredOptions) RegistryClient(bool) client.RegistryClient {
	panic("not used")
}

func (o *CredOptions) ContentTrustEnabled() bool {
	panic("not used")
}

func (o *CredOptions) ContextStore() ctxStore.Store {
	panic("not used")
}

func (o *CredOptions) CurrentContext() string {
	panic("not used")
}

func (o *CredOptions) StackOrchestrator(flagValue string) (command.Orchestrator, error) {
	panic("not used")
}

func (o *CredOptions) DockerEndpoint() docker.Endpoint {
	panic("not used")
}

func newLoginOptions(opts *opts.GlobalOptions, ios genericclioptions.IOStreams) *CredOptions {
	return &CredOptions{
		GlobalOptions: opts,
		streams:       ios,
		in:            streams.NewIn(os.Stdin),
		out:           streams.NewOut(os.Stdout),
	}
}

const dockerIndexServer = "https://index.docker.io/v1/"

func (o *CredOptions) Complete(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return xerrors.New("only one argument can be received as the registry server.")
	}

	if len(args) > 0 {
		o.server = args[0]
	}

	if len(o.server) == 0 || o.server == "docker.io" {
		o.server = dockerIndexServer
		o.isDefaultRegistry = true
	}

	if o.passwordStdin {
		if len(o.username) == 0 {
			return xerrors.New("--username is required with the password")
		}

		if len(o.password) == 0 {
			return xerrors.New("--password and --password-stdin are mutually exclusive")
		}

		contents, err := ioutil.ReadAll(o.In())
		if err != nil {
			return err
		}

		o.password = strings.TrimSuffix(string(contents), "\n")
		o.password = strings.TrimSuffix(o.password, "\r")
	}

	return nil
}

func (o *CredOptions) Validate() error {
	return nil
}

func (o *CredOptions) Login(ctx context.Context) (err error) {
	var authConfig *dockerTypes.AuthConfig
	var response registrytypes.AuthenticateOKBody
	authConfig, err = command.GetDefaultAuthConfig(
		o, o.username == "" && o.password == "", o.server, o.isDefaultRegistry,
	)
	if err == nil && authConfig.Username != "" && authConfig.Password != "" {
		response, err = loginClientSide(ctx, authConfig)
	}

	if err != nil || authConfig.Username == "" || authConfig.Password == "" {
		err = command.ConfigureAuth(o, o.username, o.password, authConfig, o.isDefaultRegistry)
		if err != nil {
			return err
		}

		response, err = loginClientSide(ctx, authConfig)
		if err != nil {
			return err
		}
	}

	if response.IdentityToken != "" {
		authConfig.Password = ""
		authConfig.IdentityToken = response.IdentityToken
	}

	creds := o.ConfigFile().GetCredentialsStore(o.server)
	if err := creds.Store(configtypes.AuthConfig(*authConfig)); err != nil {
		return errors.Errorf("Error saving credentials: %v", err)
	}

	if response.Status != "" {
		fmt.Fprintln(o.Out(), response.Status)
	}
	return nil
}

func (o *CredOptions) Logout(_ context.Context) (err error) {
	var (
		loggedIn        bool
		regsToLogout    []string
		hostnameAddress = o.server
		regsToTry       = []string{o.server}
	)
	if !o.isDefaultRegistry {
		hostnameAddress = registry.ConvertToHostname(o.server)
		regsToTry = append(regsToTry, hostnameAddress, "http://"+hostnameAddress, "https://"+hostnameAddress)
	}

	for _, s := range regsToTry {
		if _, ok := o.ConfigFile().AuthConfigs[s]; ok {
			loggedIn = true
			regsToLogout = append(regsToLogout, s)
		}
	}

	if !loggedIn {
		fmt.Fprintf(o.Out(), "Not logged in to %s\n", hostnameAddress)
		return nil
	}

	fmt.Fprintf(o.Out(), "Removing login credentials for %s\n", hostnameAddress)
	for _, r := range regsToLogout {
		if err := o.ConfigFile().GetCredentialsStore(r).Erase(r); err != nil {
			fmt.Fprintf(o.Err(), "could not erase credentials: %v\n", err)
		}
	}

	return
}

func loginClientSide(ctx context.Context, auth *dockerTypes.AuthConfig) (registrytypes.AuthenticateOKBody, error) {
	svc, err := registry.NewService(registry.ServiceOptions{})
	if err != nil {
		return registrytypes.AuthenticateOKBody{}, err
	}

	status, token, err := svc.Auth(ctx, auth, command.UserAgent())

	return registrytypes.AuthenticateOKBody{
		Status:        status,
		IdentityToken: token,
	}, err
}

func NewCmdLogin(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *cobra.Command {
	o := newLoginOptions(opts, streams)

	var cmd = &cobra.Command{
		Use:   "login [OPTIONS] [SERVER]",
		Short: "Log in to a image registry",
		Long: `Log in to a image registry. If registry server is omitted, the docker registry will be used.
The token is saved in $HOME/.docker/config.json if succeeded.

You need to login before pushing private images using the builtin builder.`,
		Example: `# Log in to the docker registry via user foo and password "bar".
kubectl dev login -u foo -p bar

# Log in in the interactive mode.
kubectl dev login
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(cmd, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Login(cmd.Context()); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&o.username, "username", "u", "", "Username")
	cmd.Flags().StringVarP(&o.password, "password", "p", "", "Password")
	cmd.Flags().BoolVar(&o.passwordStdin, "password-stdin", false, "Take the password from stdin")
	return cmd
}

func NewCmdLogout(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *cobra.Command {
	o := newLoginOptions(opts, streams)

	var cmd = &cobra.Command{
		Use:          "logout [SERVER]",
		Short:        "Log out from a image registry",
		Long:         `Log out from a image registry. If registry server is omitted, the docker registry will be used.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(cmd, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Logout(cmd.Context()); err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}
