package app

import (
	"bufio"
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/opts"
	"github.com/warm-metal/kubectl-dev/pkg/session"
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
	"io"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"net/url"
	"os"
	"time"
)

type AppOptions struct {
	*opts.GlobalOptions
	streams genericclioptions.IOStreams
	name    string

	image     string
	command   []string
	hostPaths []string
}

func (o *AppOptions) Complete(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		cmd.SilenceUsage = false
		return xerrors.Errorf("no enough parameters: %#v", args)
	}

	o.image = args[0]
	if len(o.image) == 0 {
		cmd.SilenceUsage = false
		return xerrors.Errorf("the first argument must be an image")
	}

	o.command = args[1:]
	return nil
}

func (o *AppOptions) Validate() error {
	return nil
}

func (o *AppOptions) Run() error {
	clientset, err := o.ClientSet()
	if err != nil {
		return err
	}

	endpoints, err := utils.FetchServiceEndpoints(clientset,
		o.GlobalOptions.DevNamespace, "session-gate", "session-gate")
	if err != nil {
		return err
	}

	var cc *grpc.ClientConn
	for i, ep := range endpoints {
		endpoint, err := url.Parse(ep)
		if err != nil {
			panic(err)
		}
		ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Second)
		cc, err = grpc.DialContext(ctx, endpoint.Host, grpc.WithInsecure(), grpc.WithBlock())
		cancel()
		if err == nil {
			break
		}

		fmt.Fprintf(os.Stderr, `can't connect to app session gate "%s": %s`+"\n", endpoint.Host, err)
		i++
		if i < len(endpoints) {
			fmt.Fprintf(os.Stderr, `Try the next endpoint %s`+"\n", endpoints[i])
		}
	}

	if cc == nil {
		return xerrors.Errorf("all remote endpoints are unavailable")
	}

	appCli := session.NewAppGateClient(cc)
	app, err := appCli.OpenApp(context.TODO())
	if err != nil {
		return xerrors.Errorf("can't open app session: %s", err)
	}

	err = app.Send(&session.OpenAppRequest{
		App: &session.App{
			Name:     o.name,
			Image:    o.image,
			Hostpath: o.hostPaths,
		},
		Stdin: o.command,
	})

	if err != nil {
		return xerrors.Errorf("can't open app: %s", err)
	}

	errCh := make(chan error)
	defer close(errCh)

	stdin := make(chan string)

	go func() {
		stdReader := bufio.NewReader(o.streams.In)
		defer close(stdin)
		for {
			line, prefix, err := stdReader.ReadLine()
			if err != nil && err != io.EOF {
				errCh <- xerrors.Errorf("can't read the input:%s", err)
				return
			}

			if err == io.EOF {
				return
			}

			if prefix {
				errCh <- xerrors.Errorf("line is too lang")
				return
			}

			stdin <- string(line)
		}
	}()

	stdout := make(chan string)
	stderr := make(chan string)
	go func() {
		defer close(stdout)
		defer close(stderr)
		for {
			resp, err := app.Recv()
			if err != nil {
				if err != io.EOF {
					errCh <- xerrors.Errorf("can't read the remote response:%s", err)
				} else {
					errCh <- err
				}

				return
			}

			if len(resp.Stdout) > 0 {
				stdout <- resp.Stdout
			}

			if len(resp.Stderr) > 0 {
				stderr <- resp.Stderr
			}
		}
	}()

	for {
		select {
		case err := <-errCh:
			if err == io.EOF {
				return nil
			}

			return err
		case in, ok := <-stdin:
			if ok {
				err = app.Send(&session.OpenAppRequest{
					Stdin: []string{in},
				})
				if err != nil {
					return err
				}
			}
		case out, ok := <-stdout:
			if ok {
				if _, err := o.streams.Out.Write([]byte(out)); err != nil {
					fmt.Fprintf(os.Stderr, "can't write to stdout: %s", err)
				}
			}
		case err, ok := <-stderr:
			if ok {
				if _, err := o.streams.ErrOut.Write([]byte(err)); err != nil {
					fmt.Fprintf(os.Stderr, "can't write to stderr: %s", err)
				}
			}
		}
	}
}

func NewCmd(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *cobra.Command {
	o := &AppOptions{
		GlobalOptions: opts,
		streams:       streams,
	}

	var cmd = &cobra.Command{
		Use:          "app [OPTIONS] image -- command",
		Short:        "Run an app.",
		Long:         ``,
		Example:      ``,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(cmd, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			if err := o.Run(); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&o.name, "name", "", "App name. A random name would be used if not set.")
	cmd.Flags().StringSliceVar(&o.hostPaths, "hostpath", nil, "Host paths to be mounted")

	o.AddFlags(cmd.Flags())
	return cmd
}
