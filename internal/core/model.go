package core

import (
	"fmt"
	"time"
)

type AuthType string

const (
	AuthPassword AuthType = "password"
	AuthKey      AuthType = "key"   // private key + opt. passphrase
	AuthAgent    AuthType = "agent" // ssh-agent
)

// Folders tree node
type Folder struct {
	ID       string
	ParentID *string
	Name     string
	Order    int
}

type Host struct {
	ID         string
	FolderID   *string
	Label      string // shown name
	Hostname   string
	Port       int // 22 by default
	Username   string
	Auth       AuthType
	IdentityID *string           // inline or link to reusable creds
	SecretRef  string            // vault key "host:<id>:password" / ":keypath"
	JumpHostID *string           // ProxyJump/bastion
	Options    map[string]string //keepalive, ciphers, ...
	Tags       []string
	UpdatedAt  time.Time
	DeletedAt  *time.Time // soft-deletion for sync with server
}

func (h Host) Addr() string {
	port := 22
	if h.Port != 0 {
		port = h.Port
	}
	return fmt.Sprintf("%s:%d", h.Hostname, port)
}

// Reusable creds (private keys, passwords, ...)
type Identity struct {
	ID        string
	Name      string
	Auth      AuthType
	SecretRef string
	UpdatedAt time.Time
}
