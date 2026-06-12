package sshx

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

const DefaultConnectionTimeout = 30 * time.Second

type Creds struct {
	User string
	Auth []ssh.AuthMethod
}

func Dial(addr string, c Creds, hostKeyCB ssh.HostKeyCallback) (*ssh.Client, error) {
	cfg := &ssh.ClientConfig{
		User:            c.User,
		Auth:            c.Auth,
		HostKeyCallback: hostKeyCB,
		Timeout:         DefaultConnectionTimeout,
	}
	return ssh.Dial("tcp", addr, cfg)
}

// DialJump — connection via bastion (ProxyJump).
func DialJump(jump *ssh.Client, targetAddr string, c Creds, cb ssh.HostKeyCallback) (*ssh.Client, error) {
	// Open conn to target host via bastion
	conn, err := jump.Dial("tcp", targetAddr)
	if err != nil {
		return nil, fmt.Errorf("jump dial: %w", err)
	}
	// Use bation connection as underlying to start ssh session
	cfg := &ssh.ClientConfig{User: c.User, Auth: c.Auth, HostKeyCallback: cb, Timeout: 10 * time.Second}
	ncc, chans, reqs, err := ssh.NewClientConn(conn, targetAddr, cfg)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(ncc, chans, reqs), nil
}

func parseHostPort(addr string) (string, string, error) { return net.SplitHostPort(addr) }
