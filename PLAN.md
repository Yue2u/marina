# Berth — план реализации + тренировка Go

> Лёгкая консольная утилита для управления SSH-подключениями: **TUI + CLI + E2E-синхронизация + MCP-сервер**, один статический бинарь.
>
> Документ устроен как дорожная карта. Ключевые и сложные части даны снипетами; рутину делаешь сам — она помечена **🏋️ Упражнение**. Двигайся по этапам сверху вниз: каждый этап — рабочее приложение, которое можно запустить.

---

## 0. Стек и почему так

| Компонент | Решение | Почему |
|---|---|---|
| Язык | **Go** | Один статический бинарь без рантайма, лучший SSH+TUI tooling, быстрая разработка |
| TUI | Bubble Tea + Lip Gloss + Bubbles | Elm-архитектура, предсказуемое состояние |
| CLI | Cobra | Стандарт де-факто, подкоманды |
| SSH | `golang.org/x/crypto/ssh` | Нативный, без CGO |
| Хранилище | SQLite через `modernc.org/sqlite` | Чистый Go (нет CGO → single binary), нормальный diff для sync |
| Crypto | `golang.org/x/crypto` (argon2, chacha20poly1305) | Argon2id + XChaCha20-Poly1305 |
| MCP | `github.com/modelcontextprotocol/go-sdk` | Официальный SDK, типобезопасный AddTool |
| Сервер sync | стандартный `net/http` | Тупое E2E-хранилище блобов, минимум зависимостей |

**Главный принцип архитектуры:** весь функционал — это тонкие слои (`tui`, `cli`, `mcp`, `server`) над одним пакетом `core`. Бизнес-логика живёт только в `core`. Это и правильно инженерно, и удобно для практики: один раз пишешь логику, трижды переиспользуешь.

---

## 1. Раскладка репозитория

```
berth/
  go.mod
  cmd/berth/main.go          # точка входа, диспетчеризация в cli
  internal/
    core/
      model.go               # Folder, Host, Identity, типы auth
      store/
        store.go             # интерфейс Store
        sqlite.go            # реализация на SQLite
        migrate.go           # схема БД
      vault/
        vault.go             # шифрование секретов (Argon2id + XChaCha20)
      sshx/
        dialer.go            # установка соединения, ProxyJump
        session.go           # интерактивный PTY, exec команд
        knownhosts.go        # TOFU-верификация host key
      sync/
        client.go            # push/pull
        protocol.go          # типы запросов/ответов
        merge.go             # конфликт-резолвинг
    cli/
      root.go                # cobra root
      connect.go ls.go add.go ...
    tui/
      app.go                 # главная Bubble Tea модель
      tree.go form.go        # компоненты
    mcpserver/
      server.go              # MCP-инструменты
    syncserver/
      server.go              # HTTP-сервер синхронизации
```

> **Почему `internal/`:** пакеты в `internal/` нельзя импортировать извне модуля — это твоя приватная реализация. Хорошая привычка из реальных Go-проектов.

**🏋️ Упражнение 1.0:** создай этот каркас.
```bash
mkdir berth && cd berth
go mod init github.com/<ты>/berth
# создай пустые файлы и пакеты, проверь что `go build ./...` проходит
```

---

## 2. Зависимости

```bash
go get github.com/spf13/cobra@latest
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/bubbles@latest
go get golang.org/x/crypto@latest
go get golang.org/x/term@latest
go get modernc.org/sqlite@latest
go get github.com/modelcontextprotocol/go-sdk@latest
```

> MCP SDK ещё стабилизируется (стабильный API ожидается к v1.5.0). Зафиксируй версию в `go.mod` и сверяйся с README/pkg.go.dev — сигнатуры `AddTool`/handler могли немного измениться к моменту, когда ты до этого дойдёшь.

---

## Этап 1 — Core + CLI (MVP, самый важный)

Цель этапа: из терминала добавлять/смотреть хосты и **реально подключаться** к серверу. Без TUI, без sync — только фундамент.

### 2.1 Модель данных

`internal/core/model.go`:

```go
package core

import "time"

// AuthType — способ аутентификации на хосте.
type AuthType string

const (
	AuthPassword AuthType = "password"
	AuthKey      AuthType = "key"      // приватный ключ (+ опц. passphrase)
	AuthAgent    AuthType = "agent"    // ssh-agent
)

// Folder — узел дерева. parentID == nil → корень.
type Folder struct {
	ID       string
	ParentID *string
	Name     string
	Order    int
}

// Host — один сервер.
type Host struct {
	ID         string
	FolderID   *string
	Label      string            // отображаемое имя
	Hostname   string
	Port       int               // по умолчанию 22
	Username   string
	Auth       AuthType
	IdentityID *string           // ссылка на переиспользуемые креды (или inline ниже)
	SecretRef  string            // ключ в vault: "host:<id>:password" / ":keypath"
	JumpHostID *string           // ProxyJump / bastion
	Options    map[string]string // keepalive, ciphers, ...
	Tags       []string
	UpdatedAt  time.Time
	DeletedAt  *time.Time        // soft-delete для синхронизации
}

// Identity — переиспользуемые креды (один ключ для многих хостов).
type Identity struct {
	ID        string
	Name      string
	Auth      AuthType
	SecretRef string
	UpdatedAt time.Time
}
```

> **Go-момент:** `*string` для опциональных полей — идиоматичный способ отличить «нет значения» от «пустая строка». Привыкай: nil-указатель = NULL в БД.

**🏋️ Упражнение 2.1:** добавь метод `func (h Host) Addr() string`, возвращающий `hostname:port` (с дефолтом 22, если порт 0). Это тренировка методов на значении.

### 2.2 Store — интерфейс и SQLite

Сначала **интерфейс** (тренировка одной из главных идей Go — программируй против интерфейса, не реализации):

`internal/core/store/store.go`:

```go
package store

import (
	"context"
	"github.com/<ты>/berth/internal/core"
)

type Store interface {
	Folders(ctx context.Context) ([]core.Folder, error)
	Hosts(ctx context.Context, folderID *string) ([]core.Host, error)
	Host(ctx context.Context, id string) (core.Host, error)
	SaveHost(ctx context.Context, h core.Host) error
	DeleteHost(ctx context.Context, id string) error // soft-delete: ставит DeletedAt
	// ... папки, identities аналогично
	Close() error
}
```

Реализация — `internal/core/store/sqlite.go`. Ключевой момент — driver чистый Go:

```go
package store

import (
	"context"
	"database/sql"

	_ "modernc.org/sqlite" // регистрирует драйвер под именем "sqlite"
)

type SQLiteStore struct{ db *sql.DB }

func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	s := &SQLiteStore{db: db}
	if err := s.migrate(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }
```

Схема (`migrate.go`) — простой `CREATE TABLE IF NOT EXISTS`:

```go
const schema = `
CREATE TABLE IF NOT EXISTS folders (
  id TEXT PRIMARY KEY, parent_id TEXT, name TEXT NOT NULL, sort_order INT
);
CREATE TABLE IF NOT EXISTS hosts (
  id TEXT PRIMARY KEY,
  folder_id TEXT,
  label TEXT NOT NULL,
  hostname TEXT NOT NULL,
  port INT NOT NULL DEFAULT 22,
  username TEXT NOT NULL,
  auth TEXT NOT NULL,
  identity_id TEXT,
  secret_ref TEXT,
  jump_host_id TEXT,
  options TEXT,           -- JSON
  tags TEXT,              -- JSON
  updated_at TEXT NOT NULL,
  deleted_at TEXT
);
`

func (s *SQLiteStore) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schema)
	return err
}
```

**🏋️ Упражнение 2.2:** реализуй `SaveHost` (UPSERT через `INSERT ... ON CONFLICT(id) DO UPDATE`), `Hosts` и `Host`. Подсказки:
- `options`/`tags` сериализуй через `encoding/json` в TEXT-колонку.
- Для опциональных `*string` помогает `sql.NullString`: при чтении проверяешь `.Valid`.
- `DeleteHost` не удаляет строку, а ставит `deleted_at = now()`. В `Hosts` добавь `WHERE deleted_at IS NULL`.

> Это самое объёмное упражнение этапа — зато после него ты владеешь `database/sql`, который в Go одинаков для любой БД.

### 2.3 Vault — шифрование секретов

Секреты (пароли, passphrase ключей) **никогда** не лежат в открытом виде и не уходят на сервер расшифрованными.

`internal/core/vault/vault.go`:

```go
package vault

import (
	"crypto/rand"
	"errors"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// deriveKey выводит 32-байтный ключ из мастер-пароля.
// Параметры Argon2id: time=3, memory=64MiB, threads=4 — разумный baseline.
func deriveKey(password, salt []byte) []byte {
	return argon2.IDKey(password, salt, 3, 64*1024, 4, chacha20poly1305.KeySize)
}

// Seal шифрует plaintext. На выход: salt(16) || nonce(24) || ciphertext.
func Seal(password, plaintext []byte) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key := deriveKey(password, salt)

	aead, err := chacha20poly1305.NewX(key) // XChaCha20-Poly1305, nonce 24 байта
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	// Seal(dst, nonce, plaintext, additionalData) — дописываем ct к (salt||nonce)
	out := append(salt, nonce...)
	return aead.Seal(out, nonce, plaintext, nil), nil
}
```

**🏋️ Упражнение 2.3:** напиши обратную `Open(password, blob []byte) ([]byte, error)`:
1. отрежь первые 16 байт — `salt`, следующие 24 — `nonce`, остальное — ciphertext;
2. `deriveKey`, `chacha20poly1305.NewX`;
3. `aead.Open(nil, nonce, ciphertext, nil)`. Если вернулась ошибка — это **неверный пароль или повреждённые данные**, верни понятную ошибку (`errors.New("invalid password or corrupted vault")`).

> **Go-момент:** `append(salt, nonce...)` — учись думать о слайсах. И обрати внимание: AEAD сам проверяет целостность, отдельный MAC не нужен.

Поверх `Seal/Open` сделай тип `Vault` с map `ref → encrypted bytes`, который умеет загружаться/сохраняться в `vault.enc`. Мастер-пароль спрашивай через `term.ReadPassword(int(os.Stdin.Fd()))` (без эха).

### 2.4 SSH-движок

Две задачи: **выполнить команду** (для CLI/MCP) и **интерактивная сессия** (для TUI/connect).

Верификация host key — обязательна. Делаем TOFU (trust on first use) поверх `~/.ssh/known_hosts`:

`internal/core/sshx/dialer.go`:

```go
package sshx

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

type Creds struct {
	User    string
	Auth    []ssh.AuthMethod // Password / PublicKeys / agent
}

func Dial(addr string, c Creds, hostKeyCB ssh.HostKeyCallback) (*ssh.Client, error) {
	cfg := &ssh.ClientConfig{
		User:            c.User,
		Auth:            c.Auth,
		HostKeyCallback: hostKeyCB,        // НИКОГДА не InsecureIgnoreHostKey в проде
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", addr, cfg)
}

// DialJump — подключение через bastion (ProxyJump).
func DialJump(jump *ssh.Client, targetAddr string, c Creds, cb ssh.HostKeyCallback) (*ssh.Client, error) {
	// 1. через bastion открываем TCP до целевого хоста
	conn, err := jump.Dial("tcp", targetAddr)
	if err != nil {
		return nil, fmt.Errorf("jump dial: %w", err)
	}
	// 2. поверх этого соединения поднимаем SSH к цели
	cfg := &ssh.ClientConfig{User: c.User, Auth: c.Auth, HostKeyCallback: cb, Timeout: 10 * time.Second}
	ncc, chans, reqs, err := ssh.NewClientConn(conn, targetAddr, cfg)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(ncc, chans, reqs), nil
}

func parseHostPort(addr string) (string, string, error) { return net.SplitHostPort(addr) }
```

Сборка `Auth` из нашего `Host` + `Vault`:

```go
// password:
ssh.Password(string(plainPassword))
// private key:
signer, err := ssh.ParsePrivateKey(keyBytes)            // без passphrase
signer, err := ssh.ParsePrivateKeyWithPassphrase(keyBytes, passphrase) // с passphrase
auth := ssh.PublicKeys(signer)
// agent: подключиться к $SSH_AUTH_SOCK через golang.org/x/crypto/ssh/agent
```

Интерактивная сессия с корректным PTY — самый «трюковый» кусок:

`internal/core/sshx/session.go`:

```go
package sshx

import (
	"os"

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
	// переводим локальный терминал в raw-режим, обязательно восстанавливаем
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
	return sess.Wait()
}
```

**🏋️ Упражнение 2.4:**
- Добавь `RunCommand(client, cmd string) (stdout, stderr []byte, exitCode int, err error)` через `sess.Output()` / `CombinedOutput()`. Это понадобится и для CLI, и для MCP.
- (Бонус) Обработай ресайз окна: лови `SIGWINCH` через `os/signal`, и на каждый сигнал зови `sess.WindowChange(h, w)`. Хорошая практика горутин + каналов.

### 2.5 CLI на Cobra

`cmd/berth/main.go`:

```go
package main

import (
	"fmt"
	"os"

	"github.com/<ты>/berth/internal/cli"
)

func main() {
	if err := cli.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

`internal/cli/root.go`:

```go
package cli

import "github.com/spf13/cobra"

func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "berth",
		Short: "Швартуйся к любому серверу",
	}
	root.AddCommand(connectCmd(), lsCmd(), addCmd())
	// Если запущено без подкоманды — позже сюда повесим запуск TUI.
	return root
}
```

`internal/cli/connect.go`:

```go
package cli

import "github.com/spf13/cobra"

func connectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect <host>",
		Short: "Подключиться к хосту по label/id",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. открыть store, найти Host по args[0]
			// 2. открыть vault, собрать ssh.AuthMethod
			// 3. sshx.Dial → (опц. DialJump) → sshx.InteractiveShell
			return nil // твоя реализация
		},
	}
}
```

**🏋️ Упражнение 2.5:** реализуй `ls` (вывод дерева папок/хостов), `add` (флаги `--host --user --port --auth`), `rm`. Используй `cobra` флаги (`cmd.Flags().StringVar(...)`). Это закрепит сборку команд и работу со `store`.

**✅ Контрольная точка Этапа 1:** `berth add` создаёт хост, `berth connect` открывает реальную shell-сессию на сервере. Поздравляю — у тебя уже полезная утилита.

---

## Этап 2 — TUI (Bubble Tea)

Bubble Tea — это Elm-архитектура: **Model → View → Update**. Состояние неизменяемо снаружи, всё меняется только в `Update` в ответ на сообщения (`Msg`).

`internal/tui/app.go` — скелет:

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type focus int

const (
	focusTree focus = iota
	focusForm
)

type model struct {
	focus  focus
	folders []folderNode // дерево, развёрнутое в плоский список с отступами
	cursor int
	width, height int
	err    error
}

func New(/* store, vault */) model { return model{} }

// Init — стартовая команда (напр. загрузка хостов из store).
func (m model) Init() tea.Cmd {
	return loadHostsCmd // tea.Cmd — это func() tea.Msg
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case hostsLoadedMsg:
		m.folders = msg.nodes
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "j", "down":
			if m.cursor < len(m.folders)-1 { m.cursor++ }
		case "k", "up":
			if m.cursor > 0 { m.cursor-- }
		case "enter":
			// запустить SSH-сессию для выбранного хоста
			return m, connectCmd(m.folders[m.cursor])
		}
	}
	return m, nil
}

func (m model) View() string {
	// рендер дерева слева, детали хоста справа (Lip Gloss для рамок/цветов)
	return renderTree(m)
}
```

> **Важный приём:** чтобы запустить интерактивный SSH из TUI, нужно временно «отпустить» терминал. Используй `tea.ExecProcess` — Bubble Tea приостанавливает рендер, отдаёт TTY процессу/функции и возвращает управление по завершении. Не пытайся звать `InteractiveShell` напрямую из `Update` — будет каша на экране.

```go
func connectCmd(h hostNode) tea.Cmd {
	return tea.ExecProcess(/* команда или обёртка */, func(err error) tea.Msg {
		return sessionEndedMsg{err: err}
	})
}
```

**🏋️ Упражнение 3.0 (поэтапно):**
1. Отрисуй дерево папок/хостов с курсором (`renderTree`), навигация `j/k`, сворачивание папок по `space`.
2. Подключи `bubbles/textinput` для поиска по `/`.
3. Форма создания/редактирования хоста (`focusForm`) на нескольких `textinput`.
4. `enter` на хосте → `tea.ExecProcess` → SSH-сессия → возврат в TUI.

> **Go-момент:** `Msg` — это `interface{}`, и весь `Update` строится на type switch `switch msg := msg.(type)`. Это центральный паттерн Bubble Tea и отличная тренировка type assertions.

**✅ Контрольная точка Этапа 2:** запускаешь `berth` без аргументов — открывается TUI, навигация по дереву, Enter подключает.

---

## Этап 3 — Синхронизация (E2E)

**Принцип:** сервер — тупое хранилище зашифрованных блобов. Он аутентифицирует устройство, но **не может расшифровать** содержимое (ключ только у клиента). URL опционален — без него всё работает локально.

### Протокол (`internal/core/sync/protocol.go`)

```go
package sync

type Snapshot struct {
	Version int    `json:"version"`  // монотонный, для optimistic lock
	Blob    []byte `json:"blob"`     // зашифрованный дамп store+vault (base64 в JSON)
}

type PushRequest struct {
	BaseVersion int    `json:"base_version"` // версия, от которой клиент отталкивался
	Blob        []byte `json:"blob"`
}
```

HTTP-эндпоинты сервера:

```
POST /v1/auth          → токен (по device-key или логин/пароль)
GET  /v1/snapshot      → текущий Snapshot
POST /v1/push          → принять новый блоб, если base_version == текущей (иначе 409 Conflict)
```

### Сервер (`internal/syncserver/server.go`)

```go
package syncserver

import (
	"encoding/json"
	"net/http"
	"sync"
)

type Server struct {
	mu   sync.Mutex
	snap Snapshot // в реальности — БД/файл per-user; для старта хватит этого
}

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	var req PushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.BaseVersion != s.snap.Version {
		http.Error(w, "version conflict", http.StatusConflict) // 409 → клиент делает pull+merge
		return
	}
	s.snap = Snapshot{Version: s.snap.Version + 1, Blob: req.Blob}
	json.NewEncoder(w).Encode(s.snap)
}
```

> **Go-момент:** `sync.Mutex` вокруг общего состояния и `net/http` хендлеры — фундамент любого Go-бэкенда. Заметь: стандартная библиотека закрывает 90% задач без фреймворков.

### Конфликт-резолвинг (`merge.go`)

Поскольку сервер не видит данные, мерж происходит **на клиенте**: при 409 клиент делает pull, расшифровывает удалённый снапшот, мержит со своим и пушит заново.

Старт — **last-write-wins по записям** на основе `UpdatedAt` + tombstones (`DeletedAt`):

```go
// merge сливает remote в local; при конфликте побеждает более свежий UpdatedAt.
func mergeHosts(local, remote []core.Host) []core.Host {
	byID := map[string]core.Host{}
	for _, h := range local { byID[h.ID] = h }
	for _, r := range remote {
		if cur, ok := byID[r.ID]; !ok || r.UpdatedAt.After(cur.UpdatedAt) {
			byID[r.ID] = r // включая tombstone: удаление тоже «выигрывает» по времени
		}
	}
	// ... собрать обратно в слайс
	return nil
}
```

**🏋️ Упражнение 4.0:**
- Реализуй `client.Push/Pull` (`net/http` + `encoding/json`). Блоб = `vault.Seal(masterPassword, dump)`, где `dump` — JSON всего store.
- Реализуй цикл `push → если 409 → pull → merge → push снова`.
- Подними `berth serve` как cobra-подкоманду, запускающую `syncserver`.

> **Подумай про безопасность:** аутентификация устройства на сервере (токен) **отдельна** от vault-ключа. Сервер компрометирован → утечёт только шифротекст. Это и есть E2E.

**Деплой сервера:** один бинарь → `scratch`/`distroless` Docker-образ + `docker-compose.yml`. Любой может захостить свой инстанс и указать URL в `config.toml`.

**✅ Контрольная точка Этапа 3:** меняешь хосты на машине A → `berth sync` → видишь их на машине B.

---

## Этап 4 — MCP-сервер

`berth mcp` запускает stdio MCP-сервер. Модель (Claude Desktop/Code) получает инструменты для работы с твоими серверами.

`internal/mcpserver/server.go` (API актуального официального SDK):

```go
package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Типизированные вход/выход — SDK сам генерирует JSON-схему из тегов.
type RunInput struct {
	Host    string `json:"host"    jsonschema:"label или id хоста"`
	Command string `json:"command" jsonschema:"shell-команда для выполнения"`
	Timeout int    `json:"timeout" jsonschema:"таймаут в секундах, 0 = дефолт"`
}
type RunOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

func Serve(ctx context.Context /*, deps: store, vault */) error {
	srv := mcp.NewServer(&mcp.Implementation{Name: "berth", Version: "v0.1.0"}, nil)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "run_command",
		Description: "Выполнить команду на удалённом хосте и вернуть вывод",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in RunInput) (*mcp.CallToolResult, RunOutput, error) {
		// 1. проверить allowlist: разрешён ли in.Host для модели?
		// 2. (опц.) если команда деструктивная и confirm включён → запросить подтверждение
		// 3. найти host в store, собрать creds из vault, sshx.Dial + RunCommand
		// 4. записать в audit-log
		out := RunOutput{ /* ... */ }
		return nil, out, nil
	})

	// list_hosts, upload_file, download_file — добавляются так же.

	return srv.Run(ctx, &mcp.StdioTransport{})
}
```

> Сверь точную сигнатуру handler'а с pkg.go.dev для своей версии SDK — generic `AddTool` стабилизируется, мелочи могли сдвинуться.

**Безопасность MCP (не пропускай — модель будет выполнять команды на твоих серверах):**
- **Allowlist хостов** — по умолчанию модели доступны не все хосты, а явно разрешённые в конфиге.
- **Confirm для деструктива** — опция `confirm_destructive: true`: команды вроде `rm -rf`, `dd`, `mkfs` требуют подтверждения.
- **Таймауты и лимит вывода** — чтобы один вызов не повесил сессию.
- **Audit-log** — пиши каждую выполненную через MCP команду (хост, команда, время, exit code) в отдельный файл.
- Пароли/passphrase модели не отдаём — берём из vault локально в момент подключения.

**🏋️ Упражнение 5.0:** реализуй `run_command` и `list_hosts`. Начни с allowlist и audit-log — это каркас безопасности, на который потом навешиваешь остальное.

**✅ Контрольная точка Этапа 4:** добавляешь `berth mcp` в конфиг Claude, и модель может выполнить `uname -a` на разрешённом хосте.

---

## Этап 5 — Полировка

- OS keychain для мастер-ключа (`github.com/zalando/go-keyring`) — не вводить пароль каждый раз.
- Импорт из `~/.ssh/config` (отличный онбординг; парсинг — хорошее упражнение).
- Port forwarding (local/remote) в TUI.
- Теги, фильтры, цветовые темы Lip Gloss.
- Экспорт/импорт (зашифрованный бэкап).
- `goreleaser` для сборки бинарей под все платформы + Homebrew tap.

---

## Чеклист по этапам

```
Этап 1 — Core + CLI
  [ ] каркас репозитория, go.mod
  [ ] model.go + Host.Addr()
  [ ] Store-интерфейс + SQLite (SaveHost/Hosts/Host/DeleteHost)
  [ ] vault: Seal + Open (Argon2id + XChaCha20)
  [ ] sshx: Dial, InteractiveShell, RunCommand, DialJump
  [ ] cobra: connect / ls / add / rm
  [ ] ✅ `berth connect` открывает реальную сессию

Этап 2 — TUI
  [ ] дерево + навигация j/k, сворачивание папок
  [ ] поиск (/), форма создания/редактирования
  [ ] tea.ExecProcess → SSH-сессия → возврат
  [ ] ✅ `berth` без аргументов = TUI

Этап 3 — Sync
  [ ] protocol + client (push/pull)
  [ ] syncserver (409 на конфликт версий)
  [ ] merge (LWW + tombstones)
  [ ] Docker-образ + compose
  [ ] ✅ A → sync → B

Этап 4 — MCP
  [ ] run_command + list_hosts
  [ ] allowlist + confirm + audit-log
  [ ] ✅ Claude выполняет команду на хосте

Этап 5 — полировка (по вкусу)
```

---

## Что именно ты прокачаешь в Go (по этапам)

- **Интерфейсы и композиция** — `Store` как интерфейс, тонкие слои над `core`. Главная идея языка.
- **`database/sql`** — универсальный для любой БД; `sql.NullString`, контексты.
- **Срезы и байты** — vault (`append`, нарезка salt/nonce/ciphertext).
- **`crypto/*`** — реальное прикладное шифрование, AEAD, KDF.
- **Горутины + каналы** — SIGWINCH-резайз, sync-клиент.
- **Type switch / assertions** — сердце Bubble Tea (`Update`).
- **`net/http` + `sync.Mutex`** — бэкенд на стандартной библиотеке.
- **Generics** — типобезопасный `mcp.AddTool[In, Out]`.
- **Обработка ошибок** — `fmt.Errorf("...: %w", err)`, оборачивание и `errors.Is/As`.

Начни с Этапа 1, пункт 2.1, и не переходи к TUI, пока `berth connect` не подключает по-настоящему — фундамент важнее красоты.
