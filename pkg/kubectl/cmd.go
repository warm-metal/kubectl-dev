package kubectl

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func run(args ...string) error {
	cmd := exec.Command("kubectl", args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, `fail to run "%s": %s\n`, strings.Join(args, " "), err)
	}

	return err
}
