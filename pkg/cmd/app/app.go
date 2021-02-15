package app

import (
	"bufio"
	"context"
	"fmt"
	"github.com/moby/term"
	"github.com/spf13/cobra"
	"github.com/warm-metal/kubectl-dev/pkg/cmd/opts"
	"github.com/warm-metal/kubectl-dev/pkg/session"
	"github.com/warm-metal/kubectl-dev/pkg/utils"
	"golang.org/x/sys/unix"
	"golang.org/x/xerrors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/util/exec"
	"k8s.io/klog/v2"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"time"
)

type AppOptions struct {
	*opts.GlobalOptions
	genericclioptions.IOStreams

	name      string
	namespace string

	command []string

	stdInFd, stdOutFd uintptr
	appCli            session.AppGateClient
}

func (o *AppOptions) Complete(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		cmd.SilenceUsage = false
		return xerrors.Errorf("no enough parameters: %#v", args)
	}

	if o.Raw().Namespace != nil && len(*o.Raw().Namespace) > 0 {
		o.namespace = *o.Raw().Namespace
	}

	o.command = args

	if inFd, isTerminal := term.GetFdInfo(o.In); !isTerminal {
		return xerrors.Errorf("can't execute the command without a terminal")
	} else {
		o.stdInFd = inFd
	}

	if outFd, isTerminal := term.GetFdInfo(o.Out); !isTerminal {
		return xerrors.Errorf("can't execute the command without a terminal")
	} else {
		o.stdOutFd = outFd
	}

	clientset, err := o.ClientSet()
	if err != nil {
		return err
	}

	endpoints, err := utils.FetchServiceEndpoints(clientset, "cliapp-system", "session-gate", "session-gate")
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

	o.appCli = session.NewAppGateClient(cc)
	return nil
}

func (o *AppOptions) Validate() error {
	return nil
}

func (o *AppOptions) Run() error {
	app, err := o.appCli.OpenApp(context.TODO())
	if err != nil {
		return xerrors.Errorf("can't open app session: %s", err)
	}

	err = app.Send(&session.OpenAppRequest{
		App: &session.App{
			Name:      o.name,
			Namespace: o.namespace,
		},
		Stdin:        o.command,
		TerminalSize: getSize(o.stdOutFd),
	})

	if err != nil {
		return xerrors.Errorf("can't open app: %s", err)
	}

	errCh := make(chan error)
	defer close(errCh)

	stdin := make(chan string)

	go func() {
		stdReader := bufio.NewReader(o.In)
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
	go func() {
		defer close(stdout)
		for {
			resp, err := app.Recv()
			if err != nil {
				if err == io.EOF {
					errCh <- err
					return
				}

				st, ok := status.FromError(err)
				if ok && st.Code() == codes.Aborted {
					if code, failed := strconv.Atoi(st.Message()); failed == nil {
						errCh <- exec.CodeExitError{Code: code, Err: err}
						return
					}
				}

				errCh <- xerrors.Errorf("can't read the remote response:%s", err)
				return
			}

			if len(resp.Stdout) > 0 {
				stdout <- resp.Stdout
			}
		}
	}()

	state, err := term.MakeRaw(o.stdInFd)
	if err != nil {
		return xerrors.Errorf("can't initialize terminal: %s", err)
	}

	defer func() {
		term.RestoreTerminal(o.stdInFd, state)
	}()

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, unix.SIGWINCH)
	defer signal.Stop(winch)

	for {
		select {
		case err := <-errCh:
			if err == io.EOF {
				return nil
			}

			return err
		case <-winch:
			size := getSize(o.stdOutFd)
			if err = app.Send(&session.OpenAppRequest{TerminalSize: size}); err != nil {
				return err
			}
		case in, ok := <-stdin:
			if ok {
				if err = app.Send(&session.OpenAppRequest{Stdin: []string{in}}); err != nil {
					return err
				}
			}
		case out, ok := <-stdout:
			if ok {
				fmt.Print(out)
			}
		}
	}
}

func getSize(fd uintptr) *session.TerminalSize {
	winsize, err := term.GetWinsize(fd)
	if err != nil {
		klog.Errorf("unable to get terminal size: %v", err)
		return nil
	}

	return &session.TerminalSize{Width: uint32(winsize.Width), Height: uint32(winsize.Height)}
}

func NewCmd(opts *opts.GlobalOptions, streams genericclioptions.IOStreams) *cobra.Command {
	o := &AppOptions{
		GlobalOptions: opts,
		IOStreams:     streams,
	}

	var cmd = &cobra.Command{
		Use:   "app [OPTIONS] command",
		Short: "Run a CliApp.",
		Long:  `Run a installed CliApp`,
		Example: `# Run an app

`,
		SilenceUsage:  true,
		SilenceErrors: true,
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
	o.AddFlags(cmd.Flags())

	cmd.AddCommand(
		newAppInstallCmd(opts, streams),
		newAppUninstallCmd(opts, streams),
		newAppListCmd(opts, streams),
	)
	return cmd
}
