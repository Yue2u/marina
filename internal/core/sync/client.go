package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ErrConflict возвращается когда сервер отвечает 409 — нужно pull+merge+push.
var ErrConflict = fmt.Errorf("version conflict: pull and retry")

// Client — HTTP-клиент для синхронизации.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{},
	}
}

// Pull возвращает текущий снапшот с сервера.
func (c *Client) Pull(ctx context.Context) (*Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/snapshot", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pull: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// первый push — снапшота ещё нет
		return &Snapshot{Version: 0}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pull: server returned %d", resp.StatusCode)
	}

	var snap Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		return nil, fmt.Errorf("pull decode: %w", err)
	}
	return &snap, nil
}

// Push отправляет новый блоб на сервер.
// Возвращает ErrConflict если сервер ответил 409.
func (c *Client) Push(ctx context.Context, baseVersion int, blob []byte) (*Snapshot, error) {
	body, err := json.Marshal(PushRequest{BaseVersion: baseVersion, Blob: blob})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/push", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("push: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return nil, ErrConflict
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("push: server returned %d", resp.StatusCode)
	}

	var snap Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		return nil, fmt.Errorf("push decode: %w", err)
	}
	return &snap, nil
}
