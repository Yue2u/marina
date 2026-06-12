<p align="center">
  <img src="assets/banner.svg" alt="Marina — a harbor for all your servers" width="760">
</p>

<p align="center">
  <em>A terminal-native SSH connection manager — TUI, CLI, end-to-end encrypted sync, and an MCP server for AI agents. One small static binary.</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/license-MIT-blue" alt="License">
  <img src="https://img.shields.io/badge/status-pre--alpha-orange" alt="Status">
  <img src="https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-555" alt="Platforms">
</p>

---

Marina keeps all your servers in one harbor: organized in folders, reachable in a keystroke, with secrets encrypted on your machine. It works fully offline, and when you want your hosts on a second device, it syncs them through a server that only ever sees ciphertext — host your own or use none at all.

It also speaks **MCP**, so an AI assistant can run commands on the hosts you allow — with an allowlist, confirmation for destructive actions, and a full audit log.

> **Status:** pre-alpha, in active development. The core (store, vault, SSH engine) lands first; see the [roadmap](#roadmap).

## Features

- **Folders & tags** — organize hundreds of hosts the way you think about them.
- **Every auth type** — password, SSH key (with passphrase), and `ssh-agent`. Reusable identities so one key serves many hosts.
- **TUI** — a fast keyboard-driven interface (Bubble Tea): browse, search, edit, connect.
- **CLI** — scriptable subcommands for everything the TUI does.
- **ProxyJump / bastion** — transparent multi-hop connections.
- **End-to-end encrypted sync** — the sync server is a dumb store of encrypted blobs; your master key never leaves your device. Self-hostable, and entirely optional.
- **MCP server** — let AI agents run commands on remote hosts, safely and on your terms.
- **`~/.ssh/config` import** — bring your existing hosts in seconds.
- **One binary** — no runtime, no CGO. Drop it on a box and go.

## Install

```bash
# from source (Go 1.22+)
go install github.com/<you>/marina/cmd/marina@latest

# or grab a prebuilt binary from the Releases page
```

Homebrew tap and `apt`/`scoop` packages are planned.

## Quick start

```bash
# add a host
marina add web-1 --host 203.0.113.10 --user deploy --auth key

# list your harbor
marina ls

# connect
marina connect web-1

# or just launch the TUI
marina
```

Already have hosts in your SSH config?

```bash
marina import ~/.ssh/config
```

## The TUI

Run `marina` with no arguments to open the interface.

```
┌ marina ──────────────────────────┬─ web-1 ──────────────────┐
│ ▾ production                      │ host   203.0.113.10:22   │
│     ● web-1                       │ user   deploy            │
│     ● web-2                       │ auth   key (id_ed25519)  │
│   ▾ databases                     │ jump   bastion-eu        │
│       ● pg-primary                │ tags   web, eu           │
│ ▸ staging                         │                          │
│ ▸ personal                        │ ↵ connect   e edit       │
└───────────────────────────────────┴──────────────────────────┘
  j/k move   / search   space fold   ↵ connect   e edit   q quit
```

| Key | Action |
|---|---|
| `j` / `k` | move up / down |
| `space` | fold / unfold a folder |
| `/` | fuzzy search |
| `↵` | connect to the selected host |
| `e` / `a` / `d` | edit / add / delete |
| `q` | quit |

## CLI reference

| Command | Description |
|---|---|
| `marina` | launch the TUI |
| `marina connect <host>` | open an interactive session |
| `marina ls [folder]` | list folders and hosts |
| `marina add <name> [flags]` | add a host (`--host --user --port --auth --folder`) |
| `marina rm <host>` | remove a host |
| `marina mv <host> <folder>` | move a host between folders |
| `marina import <ssh_config>` | import hosts from an SSH config |
| `marina sync [push\|pull]` | synchronize with your server |
| `marina serve` | run a sync server |
| `marina mcp` | run the MCP server over stdio |

## Sync (and self-hosting)

Marina syncs your hosts across devices **without trusting the server**. Your entire vault is encrypted locally; the server stores and version-tracks an opaque blob and nothing else.

Point Marina at any server in `config.toml`:

```toml
[sync]
url = "https://marina.example.com"
```

Run your own in one command:

```bash
marina serve --addr :8443 --data /var/lib/marina
```

Or with Docker:

```yaml
# docker-compose.yml
services:
  marina:
    image: ghcr.io/<you>/marina:latest
    command: ["serve", "--addr", ":8443"]
    ports: ["8443:8443"]
    volumes: ["marina-data:/var/lib/marina"]
volumes:
  marina-data:
```

Then `marina sync` from each device. Conflicts are resolved client-side (last-write-wins with tombstones), so the server never needs to understand your data.

## MCP — let an agent work the docks

`marina mcp` exposes your hosts to an MCP-capable assistant (e.g. Claude Desktop or Claude Code). Add it to your client config:

```json
{
  "mcpServers": {
    "marina": {
      "command": "marina",
      "args": ["mcp"]
    }
  }
}
```

Tools provided: `list_hosts`, `run_command`, `upload_file`, `download_file`.

Access is locked down by default — opt hosts in explicitly:

```toml
[mcp]
allow = ["staging-*", "web-1"]   # only these hosts are reachable by the agent
confirm_destructive = true        # rm -rf, dd, mkfs... require confirmation
audit_log = "~/.config/marina/mcp-audit.log"
```

Secrets are never handed to the model — Marina pulls them from the vault locally at connect time.

## Configuration

Everything lives under `~/.config/marina/`:

```
~/.config/marina/
├── db.sqlite      # folders, hosts, metadata
├── vault.enc      # secrets, encrypted (Argon2id + XChaCha20-Poly1305)
└── config.toml    # client settings, sync endpoint, MCP policy
```

## Security model

- **Local encryption** — passwords and key passphrases are sealed in a vault with a key derived from your master password via **Argon2id**, encrypted with **XChaCha20-Poly1305**. Plaintext secrets are never written to disk.
- **End-to-end sync** — the sync server only ever holds ciphertext. Device authentication is separate from the vault key, so a compromised server leaks nothing readable.
- **Host key verification** — connections verify host keys against `known_hosts` (trust-on-first-use), never blindly.
- **Guarded MCP** — host allowlist, confirmation for destructive commands, output limits, and an audit log of every command an agent runs.

## Roadmap

- [ ] **Core + CLI** — store, vault, SSH engine, `connect` / `ls` / `add` / `import`
- [ ] **TUI** — folder tree, search, host forms, in-TUI sessions
- [ ] **Sync** — E2E client, self-hostable server, Docker image
- [ ] **MCP** — tools, allowlist, confirmation, audit log
- [ ] **Polish** — OS keychain, port forwarding, themes, `goreleaser` builds

## Contributing

Issues and PRs are welcome. The codebase is organized as thin `tui` / `cli` / `mcp` / `server` layers over a single `core` package — start there.

## License

MIT © <you>
