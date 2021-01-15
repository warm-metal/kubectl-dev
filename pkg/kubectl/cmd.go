package kubectl

import (
	"os"
	"os/exec"
)

func runWithIO(args ...string) error {
	cmd := exec.Command("kubectl", args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func run(args ...string) error {
	// FIXME save all outputs to the popout error
	cmd := exec.Command("kubectl", args...)
	cmd.Env = os.Environ()
	return cmd.Run()
}
