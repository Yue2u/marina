package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/crypto/ssh"

	"github.com/Yue2u/marina/internal/core"
	"github.com/Yue2u/marina/internal/core/sshx"
	"github.com/Yue2u/marina/internal/core/store"
	"github.com/Yue2u/marina/internal/core/vault"
)

// Server is the MCP server that exposes SSH host tools.
type Server struct {
	st  *store.SQLiteStore
	v   *vault.Vault
	cfg Config
}

func New(st *store.SQLiteStore, v *vault.Vault, cfg Config) *Server {
	return &Server{st: st, v: v, cfg: cfg}
}

// isAllowed reports whether a host label or ID is permitted by the allowlist.
func (s *Server) isAllowed(labelOrID string) bool {
	for _, a := range s.cfg.AllowedHosts {
		if a == "*" || a == labelOrID {
			return true
		}
	}
	return false
}

func (s *Server) audit(host, cmd string, exitCode int) {
	if s.cfg.AuditLog == "" {
		return
	}
	f, err := os.OpenFile(s.cfg.AuditLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	entry := map[string]any{
		"time":      time.Now().UTC().Format(time.RFC3339),
		"host":      host,
		"command":   cmd,
		"exit_code": exitCode,
	}
	_ = json.NewEncoder(f).Encode(entry)
}

// ── tool I/O types ────────────────────────────────────────────────────────────

type ListHostsInput struct{}

type HostInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Addr  string `json:"addr"`
	User  string `json:"user"`
	Auth  string `json:"auth"`
}

type ListHostsOutput struct {
	Hosts []HostInfo `json:"hosts"`
}

type RunInput struct {
	Host    string `json:"host"              jsonschema:"label or id of the target host"`
	Command string `json:"command"           jsonschema:"shell command to execute on the remote host"`
	Timeout int    `json:"timeout,omitempty" jsonschema:"timeout in seconds; 0 uses the server default"`
}

type RunOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// ── handlers ──────────────────────────────────────────────────────────────────

func (s *Server) listHosts(ctx context.Context, _ *mcp.CallToolRequest, _ ListHostsInput) (*mcp.CallToolResult, ListHostsOutput, error) {
	hosts, err := s.st.Hosts(ctx, nil)
	if err != nil {
		return nil, ListHostsOutput{}, fmt.Errorf("list hosts: %w", err)
	}
	var out ListHostsOutput
	for _, h := range hosts {
		if s.isAllowed(h.ID) || s.isAllowed(h.Label) {
			out.Hosts = append(out.Hosts, HostInfo{
				ID:    h.ID,
				Label: h.Label,
				Addr:  h.Addr(),
				User:  h.Username,
				Auth:  string(h.Auth),
			})
		}
	}
	if out.Hosts == nil {
		out.Hosts = []HostInfo{}
	}
	return nil, out, nil
}

func (s *Server) runCommand(ctx context.Context, _ *mcp.CallToolRequest, in RunInput) (*mcp.CallToolResult, RunOutput, error) {
	if !s.isAllowed(in.Host) {
		return nil, RunOutput{}, fmt.Errorf("host %q is not in the MCP allowlist", in.Host)
	}

	hosts, err := s.st.Hosts(ctx, nil)
	if err != nil {
		return nil, RunOutput{}, fmt.Errorf("load hosts: %w", err)
	}
	var h core.Host
	var found bool
	for _, host := range hosts {
		if host.ID == in.Host || host.Label == in.Host {
			h = host
			found = true
			break
		}
	}
	if !found {
		return nil, RunOutput{}, fmt.Errorf("host %q not found", in.Host)
	}

	authMethods, err := sshx.BuildAuth(h, s.v)
	if err != nil {
		return nil, RunOutput{}, fmt.Errorf("build auth: %w", err)
	}

	cb, err := sshx.KnownHostsCallback()
	if err != nil {
		return nil, RunOutput{}, fmt.Errorf("known hosts: %w", err)
	}

	client, err := dialSSH(ctx, h, sshx.Creds{User: h.Username, Auth: authMethods}, cb, s.st)
	if err != nil {
		return nil, RunOutput{}, fmt.Errorf("connect to %s: %w", h.Addr(), err)
	}
	defer client.Close()

	timeoutSecs := in.Timeout
	if timeoutSecs <= 0 {
		timeoutSecs = s.cfg.DefaultTimeoutSecs
	}
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
	defer cancel()

	stdout, stderr, exitCode, err := runWithContext(runCtx, client, in.Command, s.cfg.MaxOutputBytes)
	if err != nil {
		return nil, RunOutput{}, err
	}

	s.audit(in.Host, in.Command, exitCode)
	return nil, RunOutput{Stdout: stdout, Stderr: stderr, ExitCode: exitCode}, nil
}

// dialSSH connects to a host, honouring JumpHostID.
func dialSSH(ctx context.Context, h core.Host, creds sshx.Creds, cb ssh.HostKeyCallback, st *store.SQLiteStore) (*ssh.Client, error) {
	if h.JumpHostID == nil {
		return sshx.Dial(h.Addr(), creds, cb)
	}
	hosts, err := st.Hosts(ctx, nil)
	if err != nil {
		return nil, err
	}
	var jump core.Host
	for _, jh := range hosts {
		if jh.ID == *h.JumpHostID {
			jump = jh
			break
		}
	}
	if jump.ID == "" {
		return nil, fmt.Errorf("jump host %q not found", *h.JumpHostID)
	}
	jumpClient, err := sshx.Dial(jump.Addr(), sshx.Creds{User: jump.Username, Auth: creds.Auth}, cb)
	if err != nil {
		return nil, fmt.Errorf("jump host dial: %w", err)
	}
	return sshx.DialJump(jumpClient, h.Addr(), creds, cb)
}

// runWithContext executes a command on the SSH client, cancelling on ctx done.
func runWithContext(ctx context.Context, client *ssh.Client, cmd string, maxBytes int) (stdout, stderr string, exitCode int, err error) {
	sess, err := client.NewSession()
	if err != nil {
		return "", "", -1, err
	}
	defer sess.Close()

	var outBuf, errBuf limitedBuffer
	outBuf.limit = maxBytes
	errBuf.limit = maxBytes
	sess.Stdout = &outBuf
	sess.Stderr = &errBuf

	if err := sess.Start(cmd); err != nil {
		return "", "", -1, fmt.Errorf("start: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- sess.Wait() }()

	select {
	case <-ctx.Done():
		sess.Signal(ssh.SIGKILL)
		<-done
		return outBuf.String(), errBuf.String(), -1, ctx.Err()
	case runErr := <-done:
		code := 0
		if runErr != nil {
			var exitErr *ssh.ExitError
			if errors.As(runErr, &exitErr) {
				code = exitErr.ExitStatus()
				runErr = nil
			}
		}
		return outBuf.String(), errBuf.String(), code, runErr
	}
}

// limitedBuffer caps total written bytes at limit (excess is silently dropped).
type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.Len()
	if remaining <= 0 {
		return len(p), nil
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	return b.Buffer.Write(p)
}

// ── entry point ───────────────────────────────────────────────────────────────

// Serve starts the MCP stdio server and blocks until the client disconnects or ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	srv := mcp.NewServer(&mcp.Implementation{Name: "marina", Version: "v0.1.0"}, nil)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_hosts",
		Description: "List SSH hosts that the model is allowed to access",
	}, s.listHosts)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "run_command",
		Description: "Execute a shell command on a remote SSH host and return stdout, stderr and exit code",
	}, s.runCommand)
	return srv.Run(ctx, &mcp.StdioTransport{})
}
