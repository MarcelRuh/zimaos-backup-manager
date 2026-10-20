package zimaos

import (
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
)

type Client struct {
	mu         sync.RWMutex
	baseURL    string
	token      string
	httpClient *http.Client
	lastOK     time.Time
	lastError  string
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) Configure(baseURL, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = strings.TrimRight(baseURL, "/")
	c.token = token
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

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		data, err := c.doOnce(ctx, method, path, body)
		if err == nil {
			return data, nil
		}
		lastErr = err
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			break
		}
		// Don't retry 4xx except 429
		if httpErr, ok := err.(*HTTPError); ok && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 && httpErr.StatusCode != 429 {
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
		c.setError(fmt.Sprintf("http %d", resp.StatusCode))
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
