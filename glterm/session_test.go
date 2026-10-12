//go:build unix

package glterm

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

// A shell on the session's pty draws on its terminal, and what is typed
// reaches it.
func TestSessionRunsAShell(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	s, err := Start(sh, nil, 40, 5)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close() //nolint:errcheck // the test is over
	s.Input("echo glterm-$((20+22))\r")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s.Lock()
		buf := s.Term.Buffer()
		var screen strings.Builder
		for i := 0; i < buf.Lines.Length(); i++ {
			screen.WriteString(buf.Lines.Get(i).TranslateToString(true, 0, 40) + "\n")
		}
		s.Unlock()
		if strings.Contains(screen.String(), "glterm-42") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the shell's output never reached the terminal")
}
