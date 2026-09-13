package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is injected at build time via -ldflags, see Makefile.
var version = "dev"

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	}
}
