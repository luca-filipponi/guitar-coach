package cli

import (
	"github.com/spf13/cobra"

	"github.com/luca-filipponi/guitar-coach/internal/server"
)

func (sh *Shell) newServeCmd() *cobra.Command {
	var port string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "start the local web UI",
		Long: `Serves the web UI for the current data directory.

Point it at another data directory with --dir (the flag works on every
command): guitar-coach serve --dir /tmp/ex-demo --port 8081
or set the GUITAR_COACH_DIR environment variable. Each --dir can run
on its own port simultaneously, so real and demo data never mix.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return server.Listen(":"+port, sh.api, sh.sig)
		},
		ValidArgsFunction: sh.noCompletion,
	}
	cmd.Flags().StringVar(&port, "port", "8080", "port to listen on")
	return cmd
}
