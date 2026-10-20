package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Client struct {
	httpClient *http.Client
	mu         sync.Mutex
	lastSent   time.Time
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type EmbedFooter struct {
	Text string `json:"text"`
}

type Embed struct {
	Title     string       `json:"title"`
	Color     int          `json:"color"`
	Fields    []EmbedField `json:"fields"`
	Footer    *EmbedFooter `json:"footer,omitempty"`
	Timestamp string       `json:"timestamp,omitempty"`
}

type webhookPayload struct {
	Embeds []Embed `json:"embeds"`
}

const (
	ColorGreen = 0x22C55E
	ColorRed   = 0xEF4444
)

func (c *Client) SendEmbed(ctx context.Context, webhookURL string, embed Embed) error {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return fmt.Errorf("webhook url empty")
	}
	if !isValidWebhookURL(webhookURL) {
		return fmt.Errorf("invalid discord webhook url")
	}

	c.mu.Lock()
	wait := time.Until(c.lastSent.Add(600 * time.Millisecond))
	c.mu.Unlock()
	if wait > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}

	body, _ := json.Marshal(webhookPayload{Embeds: []Embed{embed}})
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		c.mu.Lock()
		c.lastSent = time.Now()
		c.mu.Unlock()
		if resp.StatusCode == 429 {
			lastErr = fmt.Errorf("discord rate limited")
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("discord webhook http %d", resp.StatusCode)
		}
		return nil
	}
	return lastErr
}

func SuccessEmbed(backupName, duration, files, dataAmount string, completedAt time.Time) Embed {
	return Embed{
		Title: "✅ Backup erfolgreich abgeschlossen",
		Color: ColorGreen,
		Fields: []EmbedField{
			{Name: "Backup", Value: backupName},
			{Name: "Dauer", Value: duration},
			{Name: "Dateien", Value: files},
			{Name: "Datenmenge", Value: dataAmount},
		},
		Footer:    &EmbedFooter{Text: "ZimaOS Backup Manager"},
		Timestamp: completedAt.UTC().Format(time.RFC3339),
	}
}

func FailureEmbed(backupName, duration, files, dataAmount, errMsg string, completedAt time.Time) Embed {
	if strings.TrimSpace(errMsg) == "" {
		errMsg = "Keine Fehlermeldung von der ZimaOS-API verfügbar."
	}
	return Embed{
		Title: "❌ Backup fehlgeschlagen",
		Color: ColorRed,
		Fields: []EmbedField{
			{Name: "Backup", Value: backupName},
			{Name: "Dauer", Value: duration},
			{Name: "Dateien", Value: files},
			{Name: "Datenmenge", Value: dataAmount},
			{Name: "Fehlermeldung", Value: truncate(errMsg, 900)},
		},
		Footer:    &EmbedFooter{Text: "ZimaOS Backup Manager"},
		Timestamp: completedAt.UTC().Format(time.RFC3339),
	}
}

func TestEmbed() Embed {
	return Embed{
		Title: "✅ Discord-Verbindung erfolgreich",
		Color: ColorGreen,
		Fields: []EmbedField{
			{Name: "Backup", Value: "Testbenachrichtigung"},
			{Name: "Dauer", Value: "—"},
			{Name: "Dateien", Value: "—"},
			{Name: "Datenmenge", Value: "—"},
		},
		Footer:    &EmbedFooter{Text: "ZimaOS Backup Manager"},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

func isValidWebhookURL(webhookURL string) bool {
	if !strings.HasPrefix(webhookURL, "https://") {
		return false
	}
	return strings.Contains(webhookURL, "/api/webhooks/")
}

func MaskWebhook(url string) string {
	if url == "" {
		return ""
	}
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return "***"
	}
	parts[len(parts)-1] = "***"
	return strings.Join(parts, "/")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
