package kubectl

import (
	"io"
	"os"
	"os/exec"
)

type RunOptions func(*exec.Cmd)

func WithStdin(stdin io.Reader) RunOptions {
	return func(cmd *exec.Cmd) {
		cmd.Stdin = stdin
	}
}

func AttachIO() RunOptions {
	return func(cmd *exec.Cmd) {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
}

func runWithIO(args []string) error {
	return run(args, AttachIO())
}

func run(args []string, opts ...RunOptions) error {
	cmd := exec.Command("kubectl", args...)
	cmd.Env = os.Environ()
	for _, opt := range opts {
		opt(cmd)
	}
	return cmd.Run()
}
