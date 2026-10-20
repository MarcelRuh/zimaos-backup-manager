package zimaos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	ErrUnavailable = errors.New("zimaos api unavailable")
	ErrNotFound    = errors.New("backup task not found")
	ErrBadResponse = errors.New("invalid zimaos api response")
	ErrUnauthorized = errors.New("zimaos api unauthorized")
)

// TokenPersister optionally stores a refreshed access token.
type TokenPersister func(ctx context.Context, token string) error

type Client struct {
	mu         sync.RWMutex
	baseURL    string
	token      string
	username   string
	password   string
	persist    TokenPersister
	httpClient *http.Client
	lastOK     time.Time
	lastError  string
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   normalizeToken(token),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) Configure(baseURL, token, username, password string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = strings.TrimRight(baseURL, "/")
	c.token = normalizeToken(token)
	c.username = strings.TrimSpace(username)
	c.password = password
}

func (c *Client) SetTokenPersister(fn TokenPersister) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.persist = fn
}

func (c *Client) Status() (connected bool, lastOK time.Time, lastError string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.lastOK.IsZero() && c.lastError == "", c.lastOK, c.lastError
}

func (c *Client) ListTasks(ctx context.Context) ([]Task, error) {
	body, err := c.do(ctx, http.MethodGet, "/v2/backup/tasks", nil)
	if err != nil {
		return nil, err
	}
	var resp tasksResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadResponse, err)
	}
	if resp.Data == nil {
		return []Task{}, nil
	}
	return resp.Data, nil
}

func (c *Client) GetTask(ctx context.Context, id string) (*Task, error) {
	tasks, err := c.ListTasks(ctx)
	if err != nil {
		return nil, err
	}
	for i := range tasks {
		if tasks[i].ID == id {
			return &tasks[i], nil
		}
	}
	return nil, ErrNotFound
}

func (c *Client) StartTask(ctx context.Context, id string) (string, error) {
	path := fmt.Sprintf("/v2/backup/tasks/%s/start", id)
	body, err := c.do(ctx, http.MethodPost, path, nil)
	if err != nil {
		return "", err
	}
	var resp startResponse
	if len(body) > 0 {
		_ = json.Unmarshal(body, &resp)
	}
	if resp.Message == "" {
		resp.Message = strings.TrimSpace(string(body))
	}
	return resp.Message, nil
}

// Login obtains a temporary access_token via the confirmed ZimaOS/CasaOS user API.
// POST /v1/users/login
func (c *Client) Login(ctx context.Context) (string, error) {
	c.mu.RLock()
	base := c.baseURL
	username := c.username
	password := c.password
	c.mu.RUnlock()

	if base == "" {
		return "", fmt.Errorf("%w: base url empty", ErrUnavailable)
	}
	if username == "" || password == "" {
		return "", fmt.Errorf("%w: username/password required for login", ErrUnauthorized)
	}

	payload, _ := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/users/login", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.setError(err.Error())
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(data))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		c.setError("login failed")
		return "", fmt.Errorf("%w: login http %d: %s", ErrUnauthorized, resp.StatusCode, msg)
	}

	token, err := extractAccessToken(data)
	if err != nil {
		c.setError("login token missing")
		return "", err
	}
	token = normalizeToken(token)

	c.mu.Lock()
	c.token = token
	persist := c.persist
	c.mu.Unlock()

	if persist != nil {
		_ = persist(ctx, token)
	}
	return token, nil
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	const maxAttempts = 3
	var lastErr error
	retriedAuth := false

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		data, err := c.doOnce(ctx, method, path, body)
		if err == nil {
			return data, nil
		}
		lastErr = err

		var httpErr *HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == 401 && !retriedAuth {
			retriedAuth = true
			if _, loginErr := c.ensureAuthenticated(ctx); loginErr == nil {
				continue
			} else {
				lastErr = loginErr
			}
			break
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			break
		}
		if httpErr != nil && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 && httpErr.StatusCode != 429 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * 300 * time.Millisecond):
		}
	}
	return nil, lastErr
}

func (c *Client) ensureAuthenticated(ctx context.Context) (string, error) {
	c.mu.RLock()
	token := c.token
	hasCreds := c.username != "" && c.password != ""
	c.mu.RUnlock()

	if token != "" && !hasCreds {
		// Token present but rejected; without credentials we cannot refresh.
		c.setError("http 401 – ZimaOS-API-Token ungültig/abgelaufen; Benutzer+Passwort hinterlegen")
		return "", fmt.Errorf("%w: token rejected and no credentials configured", ErrUnauthorized)
	}
	if !hasCreds {
		c.setError("http 401 – ZimaOS-Login erforderlich (Einstellungen → Benutzer/Passwort oder Token)")
		return "", fmt.Errorf("%w: no token and no credentials", ErrUnauthorized)
	}
	return c.Login(ctx)
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("zimaos api http %d: %s", e.StatusCode, e.Body)
}

func (c *Client) doOnce(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	c.mu.RLock()
	base := c.baseURL
	token := c.token
	c.mu.RUnlock()

	if base == "" {
		c.setError("api base url not configured")
		return nil, fmt.Errorf("%w: base url empty", ErrUnavailable)
	}

	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		c.setError(err.Error())
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		// ZimaOS expects the raw JWT in Authorization (no "Bearer " prefix).
		req.Header.Set("Authorization", token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.setError(err.Error())
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		c.setError(err.Error())
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(data))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		if resp.StatusCode == 401 {
			c.setError("http 401 – Unauthorized (Token/Login nötig)")
		} else {
			c.setError(fmt.Sprintf("http %d", resp.StatusCode))
		}
		return nil, &HTTPError{StatusCode: resp.StatusCode, Body: msg}
	}

	c.mu.Lock()
	c.lastOK = time.Now().UTC()
	c.lastError = ""
	c.mu.Unlock()
	return data, nil
}

func (c *Client) setError(msg string) {
	c.mu.Lock()
	c.lastError = msg
	c.mu.Unlock()
}

func normalizeToken(token string) string {
	token = strings.TrimSpace(token)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	return token
}

func extractAccessToken(body []byte) (string, error) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return "", fmt.Errorf("%w: invalid login json", ErrBadResponse)
	}
	if token := findStringKey(root, "access_token"); token != "" {
		return token, nil
	}
	return "", fmt.Errorf("%w: access_token not found in login response", ErrBadResponse)
}

func findStringKey(v any, key string) string {
	switch t := v.(type) {
	case map[string]any:
		if raw, ok := t[key]; ok {
			if s, ok := raw.(string); ok && s != "" {
				return s
			}
		}
		for _, child := range t {
			if s := findStringKey(child, key); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range t {
			if s := findStringKey(child, key); s != "" {
				return s
			}
		}
	}
	return ""
}
