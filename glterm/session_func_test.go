//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package glterm

import (
	"bufio"
	"os"
	"testing"
	"time"

	"golang.org/x/term"
)

func waitDone(t *testing.T, s *Session) {
	t.Helper()
	select {
	case <-s.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the session did not end")
	}
}

// A cooked terminal turns a typed Ctrl+C into a signal for the program, as
// the kernel would for a program started on it.
func TestStartFuncInterrupt(t *testing.T) {
	got := make(chan os.Signal, 1)
	s, err := StartFunc(func(tty *os.File, sig <-chan os.Signal) int {
		tty.WriteString("ready\r\n") //nolint:errcheck,gosec
		got <- <-sig
		return 0
	}, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	s.Input("\x03")
	select {
	case g := <-got:
		if g != os.Interrupt {
			t.Fatalf("got %v", g)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no signal for Ctrl+C")
	}
	waitDone(t, s)
}

// In raw mode Ctrl+C is a key: the program reads it, and no signal is sent.
func TestStartFuncRawCtrlC(t *testing.T) {
	read := make(chan byte, 1)
	signaled := make(chan struct{}, 1)
	s, err := StartFunc(func(tty *os.File, sig <-chan os.Signal) int {
		old, err := term.MakeRaw(int(tty.Fd()))
		if err != nil {
			return 1
		}
		defer term.Restore(int(tty.Fd()), old)  //nolint:errcheck
		tty.WriteString("raw\r\n")              //nolint:errcheck,gosec
		b, _ := bufio.NewReader(tty).ReadByte() //nolint:errcheck
		select {
		case <-sig:
			signaled <- struct{}{}
		default:
		}
		read <- b
		return 0
	}, 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	s.Input("\x03")
	select {
	case b := <-read:
		if b != 0x03 {
			t.Fatalf("read %#x", b)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the program did not read Ctrl+C")
	}
	select {
	case <-signaled:
		t.Fatal("Ctrl+C in raw mode was also sent as a signal")
	default:
	}
	waitDone(t, s)
}
