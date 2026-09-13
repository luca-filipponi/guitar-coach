package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/luca-filipponi/guitar-coach/internal/cli"
	"github.com/luca-filipponi/guitar-coach/internal/store"
)

func main() {
	args := os.Args[1:]
	dir := dataDir(args)
	args = stripDirArg(args)

	st, err := store.Open(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	root := cli.New(st).Root()
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func dataDir(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "--dir" && i+1 < len(args) {
			return args[i+1]
		}
	}
	if dir := os.Getenv("GUITAR_COACH_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "./data"
	}
	return filepath.Join(home, ".guitar-coach")
}

func stripDirArg(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--dir" && i+1 < len(args) {
			i++
			continue
		}
		out = append(out, args[i])
	}
	return out
}
