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
	ColorGreen = 0x57F287
	ColorRed   = 0xED4245
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
			if err := sleepCtx(ctx, time.Duration(attempt)*time.Second); err != nil {
				return err
			}
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		c.mu.Lock()
		c.lastSent = time.Now()
		c.mu.Unlock()
		if resp.StatusCode == 429 {
			lastErr = fmt.Errorf("discord rate limited")
			if err := sleepCtx(ctx, time.Duration(attempt)*2*time.Second); err != nil {
				return err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("discord webhook http %d", resp.StatusCode)
		}
		return nil
	}
	return lastErr
}

// SuccessEmbed matches the compact ZimaOS Backup Discord layout.
func SuccessEmbed(backupName, taskID, duration, files, dataAmount string, completedAt time.Time, loc *time.Location) Embed {
	return Embed{
		Title: "✅ Backup erfolgreich abgeschlossen",
		Color: ColorGreen,
		Fields: []EmbedField{
			{Name: "📦 Backup", Value: backupName, Inline: true},
			{Name: "⏱️ Dauer", Value: duration, Inline: true},
			{Name: "📁 Dateien", Value: files, Inline: true},
			{Name: "💾 Datenmenge", Value: dataAmount, Inline: false},
		},
		Footer:    &EmbedFooter{Text: FormatFooter(taskID, completedAt, loc)},
		Timestamp: completedAt.UTC().Format(time.RFC3339),
	}
}

func FailureEmbed(backupName, taskID, duration, files, dataAmount, errMsg string, completedAt time.Time, loc *time.Location) Embed {
	if strings.TrimSpace(errMsg) == "" {
		errMsg = "Keine Fehlermeldung von der ZimaOS-API verfügbar."
	}
	return Embed{
		Title: "❌ Backup fehlgeschlagen",
		Color: ColorRed,
		Fields: []EmbedField{
			{Name: "📦 Backup", Value: backupName, Inline: true},
			{Name: "⏱️ Dauer", Value: duration, Inline: true},
			{Name: "📁 Dateien", Value: files, Inline: true},
			{Name: "💾 Datenmenge", Value: dataAmount, Inline: false},
			{Name: "⚠️ Fehlermeldung", Value: truncate(errMsg, 900), Inline: false},
		},
		Footer:    &EmbedFooter{Text: FormatFooter(taskID, completedAt, loc)},
		Timestamp: completedAt.UTC().Format(time.RFC3339),
	}
}

func ComposeSuccessEmbed(backupName, taskID string, okCount, failCount int, at time.Time, loc *time.Location) Embed {
	if strings.TrimSpace(backupName) == "" {
		backupName = "—"
	}
	return Embed{
		Title: "✅ Compose → AppData erfolgreich",
		Color: ColorGreen,
		Fields: []EmbedField{
			{Name: "📦 Nach Backup", Value: backupName, Inline: true},
			{Name: "🧩 Apps OK", Value: fmt.Sprintf("%d", okCount), Inline: true},
			{Name: "⚠️ Fehler", Value: fmt.Sprintf("%d", failCount), Inline: true},
			{Name: "📁 Ziel", Value: "AppData/*/docker-compose.yml", Inline: false},
		},
		Footer:    &EmbedFooter{Text: FormatFooter(taskID, at, loc)},
		Timestamp: at.UTC().Format(time.RFC3339),
	}
}

func ComposeFailureEmbed(backupName, taskID string, okCount, failCount int, errMsg string, at time.Time, loc *time.Location) Embed {
	if strings.TrimSpace(backupName) == "" {
		backupName = "—"
	}
	if strings.TrimSpace(errMsg) == "" {
		errMsg = "Compose-Sync nach Backup fehlgeschlagen."
	}
	return Embed{
		Title: "❌ Compose → AppData fehlgeschlagen",
		Color: ColorRed,
		Fields: []EmbedField{
			{Name: "📦 Nach Backup", Value: backupName, Inline: true},
			{Name: "🧩 Apps OK", Value: fmt.Sprintf("%d", okCount), Inline: true},
			{Name: "⚠️ Fehler", Value: fmt.Sprintf("%d", failCount), Inline: true},
			{Name: "🚨 Meldung", Value: truncate(errMsg, 900), Inline: false},
		},
		Footer:    &EmbedFooter{Text: FormatFooter(taskID, at, loc)},
		Timestamp: at.UTC().Format(time.RFC3339),
	}
}

func TestEmbed(loc *time.Location) Embed {
	now := time.Now()
	return Embed{
		Title: "✅ Discord-Verbindung erfolgreich",
		Color: ColorGreen,
		Fields: []EmbedField{
			{Name: "📦 Backup", Value: "Testbenachrichtigung", Inline: true},
			{Name: "⏱️ Dauer", Value: "00:00:00", Inline: true},
			{Name: "📁 Dateien", Value: "—", Inline: true},
			{Name: "💾 Datenmenge", Value: "—", Inline: false},
		},
		Footer:    &EmbedFooter{Text: FormatFooter("test", now, loc)},
		Timestamp: now.UTC().Format(time.RFC3339),
	}
}

// FormatFooter builds: "ZimaOS Backup • Task 4 • heute um 16:34 Uhr"
func FormatFooter(taskID string, at time.Time, loc *time.Location) string {
	if loc == nil {
		loc = time.Local
	}
	local := at.In(loc)
	taskPart := "Task"
	if strings.TrimSpace(taskID) != "" {
		taskPart = "Task " + strings.TrimSpace(taskID)
	}
	return fmt.Sprintf("ZimaOS Backup • %s • %s", taskPart, formatGermanRelativeTime(local, time.Now().In(loc)))
}

func formatGermanRelativeTime(at, now time.Time) string {
	ay, am, ad := at.Date()
	ny, nm, nd := now.Date()
	clock := at.Format("15:04")
	switch {
	case ay == ny && am == nm && ad == nd:
		return fmt.Sprintf("heute um %s Uhr", clock)
	case ay == ny && am == nm && ad == nd-1:
		return fmt.Sprintf("gestern um %s Uhr", clock)
	case ay == ny && am == nm && ad == nd+1:
		return fmt.Sprintf("morgen um %s Uhr", clock)
	default:
		return fmt.Sprintf("%02d.%02d.%d um %s Uhr", at.Day(), int(at.Month()), at.Year(), clock)
	}
}

// FormatDurationHHMMSS formats milliseconds as 00:00:02.
func FormatDurationHHMMSS(ms *int64) string {
	if ms == nil {
		return "00:00:00"
	}
	total := *ms / 1000
	if total < 0 {
		total = 0
	}
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// FormatDataAmount formats transferred/total like "1.01 / 1.01 GiB".
func FormatDataAmount(xferred, total int64) string {
	unit, div := pickByteUnit(max64(total, xferred))
	if div == 1 {
		return fmt.Sprintf("%d / %d %s", xferred, total, unit)
	}
	return fmt.Sprintf("%.2f / %.2f %s", float64(xferred)/float64(div), float64(total)/float64(div), unit)
}

func FormatFiles(xferred, total int64) string {
	return fmt.Sprintf("%d / %d", xferred, total)
}

func pickByteUnit(n int64) (string, int64) {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
		tib = 1024 * gib
	)
	switch {
	case n >= tib:
		return "TiB", tib
	case n >= gib:
		return "GiB", gib
	case n >= mib:
		return "MiB", mib
	case n >= kib:
		return "KiB", kib
	default:
		return "B", 1
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
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
