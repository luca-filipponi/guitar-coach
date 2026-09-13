package cli

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

const defaultPort = 8080

// openSessionInBrowser opens the web UI on the session review page. If no
// server is running on defaultPort, it starts one in the background against
// the same data dir as the current command.
func (sh *Shell) openSessionInBrowser(sessionID string) error {
	url := fmt.Sprintf("http://localhost:%d/sessions/%s", defaultPort, sessionID)
	if !serverUp(defaultPort) {
		logPath := filepath.Join(sh.st.Dir(), "serve.log")
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()

		bin, err := os.Executable()
		if err != nil {
			return err
		}
		cmd := exec.Command(bin, "--dir", sh.st.Dir(), "serve", "--port", strconv.Itoa(defaultPort))
		cmd.Stdout = f
		cmd.Stderr = f
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("starting web UI: %w", err)
		}
		_ = cmd.Process.Release()

		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) && !serverUp(defaultPort) {
			time.Sleep(100 * time.Millisecond)
		}
		if !serverUp(defaultPort) {
			return fmt.Errorf("web UI did not start (see %s)", logPath)
		}
	}
	return openURL(url)
}

func serverUp(port int) bool {
	client := &http.Client{Timeout: 300 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/", port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("opening a browser is not supported on %s", runtime.GOOS)
	}
	return cmd.Start()
}
