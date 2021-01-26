package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/warm-metal/kubectl-dev/pkg/release"
)

func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version information.",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(release.Version, release.Commit)
		},
	}
}
