package sshx

import (
	"bytes"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func InteractiveShell(client *ssh.Client) error {
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()

	fd := int(os.Stdin.Fd())

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer term.Restore(fd, oldState)

	w, h, _ := term.GetSize(fd)
	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 14400, ssh.TTY_OP_OSPEED: 14400}
	if err := sess.RequestPty("xterm-256color", h, w, modes); err != nil {
		return err
	}

	sess.Stdin, sess.Stdout, sess.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := sess.Shell(); err != nil {
		return err
	}

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)

	go func() {
		for range winch {
			w, h, _ := term.GetSize(fd)
			sess.WindowChange(h, w)
		}
	}()

	return sess.Wait()
}

func RunCommand(client *ssh.Client, cmd string) (stdout, stderr []byte, exitCode int, err error) {
	sess, err := client.NewSession()
	if err != nil {
		return nil, nil, -1, err
	}
	defer sess.Close()

	var outBuf, errBuf bytes.Buffer
	sess.Stdout = &outBuf
	sess.Stderr = &errBuf

	err = sess.Run(cmd)
	exitCode = 0
	if err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitStatus()
			err = nil // non-zero exit is not transport error
		}
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode, err
}
