package dockerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultSock      = "/var/run/docker.sock"
	apiVersion       = "v1.44"
	labelConfigFiles = "com.docker.compose.project.config_files"
	labelProject     = "com.docker.compose.project"
)

type Mount struct {
	Type        string `json:"Type"`
	Source      string `json:"Source"`
	Destination string `json:"Destination"`
}

type Container struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Labels map[string]string `json:"Labels"`
	Mounts []Mount           `json:"Mounts"`
	State  string            `json:"State"`
}

func (c Container) Name() string {
	for _, n := range c.Names {
		n = strings.TrimPrefix(n, "/")
		if n != "" {
			return n
		}
	}
	if len(c.ID) >= 12 {
		return c.ID[:12]
	}
	return c.ID
}

func (c Container) ComposeConfigFiles() []string {
	if c.Labels == nil {
		return nil
	}
	raw := strings.TrimSpace(c.Labels[labelConfigFiles])
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (c Container) ComposeProject() string {
	if c.Labels == nil {
		return ""
	}
	return strings.TrimSpace(c.Labels[labelProject])
}

type Client struct {
	sock string
	http *http.Client
}

func New(sock string) *Client {
	sock = strings.TrimSpace(sock)
	if sock == "" {
		sock = DefaultSock
	}
	sock = strings.TrimPrefix(sock, "unix://")
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		},
	}
	return &Client{
		sock: sock,
		http: &http.Client{Transport: transport, Timeout: 20 * time.Second},
	}
}

func (c *Client) Available(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/"+apiVersion+"/_ping", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func (c *Client) ListContainers(ctx context.Context) ([]Container, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/"+apiVersion+"/containers/json?all=true", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker list containers: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docker list containers: http %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var list []Container
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("decode containers: %w", err)
	}
	return list, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
