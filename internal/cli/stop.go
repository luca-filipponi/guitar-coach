package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

func (sh *Shell) newStopCmd() *cobra.Command {
	var port string
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "stop the local web UI server",
		Long: `Stops a guitar-coach web UI server listening on PORT (default 8080).

It finds the process holding the port open, sends SIGTERM for a graceful
shutdown (the same signal Ctrl-C delivers), and escalates to SIGKILL if the
server does not exit within a few seconds.

Target a server running on another data dir with --dir plus --port, e.g.:
  guitar-coach --dir /tmp/ex-demo stop --port 8081
`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return stopServer(port)
		},
		ValidArgsFunction: sh.noCompletion,
	}
	cmd.Flags().StringVar(&port, "port", strconv.Itoa(defaultPort), "port the server listens on")
	return cmd
}

// stopServer stops every guitar-coach server listening on port, using
// lsof to find the holding process (lsof ships with macOS and most Linuxes).
func stopServer(port string) error {
	pids, err := listeningPIDs(port)
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		fmt.Printf("no guitar-coach server is listening on port %s\n", port)
		return nil
	}
	for _, pid := range pids {
		if stopped, err := stopPID(pid); err != nil {
			return err
		} else if stopped {
			fmt.Printf("stopped guitar-coach server (pid %d) on port %s\n", pid, port)
		} else {
			fmt.Printf("no guitar-coach process running as pid %d on port %s\n", pid, port)
		}
	}
	return nil
}

// listeningPIDs returns the numeric PIDs of guitar-coach processes with a
// listener open on port, filtering out anything that is not one of ours.
func listeningPIDs(port string) ([]int, error) {
	if _, err := exec.LookPath("lsof"); err != nil {
		return nil, errors.New("stop needs lsof (available by default on macOS and Linux)")
	}
	out, err := exec.Command("lsof", "-nP", "-iTCP:"+port, "-sTCP:LISTEN", "-t").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && strings.TrimSpace(string(out)) == "" {
			return nil, nil
		}
		return nil, fmt.Errorf("finding the server on port %s: %w", port, err)
	}
	var pids []int
	for _, line := range strings.Fields(string(out)) {
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		if isGuitarCoachProcess(pid) {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func isGuitarCoachProcess(pid int) bool {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "guitar-coach")
}

// stopPID sends SIGTERM and waits for the process to exit, falling back to
// SIGKILL. It reports whether the process was actually stopped.
func stopPID(pid int) (bool, error) {
	if !processAlive(pid) {
		return false, nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return false, fmt.Errorf("sending SIGTERM to pid %d: %w", pid, err)
	}
	if err := waitForExit(pid, 4*time.Second); err != nil {
		fmt.Printf("server (pid %d) ignored SIGTERM, sending SIGKILL\n", pid)
		_ = proc.Kill()
		_ = waitForExit(pid, 2*time.Second)
	}
	return !processAlive(pid), nil
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func waitForExit(pid int, d time.Duration) error {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("process still running")
}
