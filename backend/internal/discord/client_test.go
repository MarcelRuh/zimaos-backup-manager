package discord

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSendEmbedSuccessAndRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	c := NewClient()
	c.httpClient = srv.Client()
	url := srv.URL + "/api/webhooks/123/token"
	if err := c.SendEmbed(context.Background(), url, TestEmbed(time.UTC)); err != nil {
		t.Fatal(err)
	}
	if attempts < 2 {
		t.Fatalf("expected retry, attempts=%d", attempts)
	}
}

func TestInvalidWebhookRejected(t *testing.T) {
	c := NewClient()
	if err := c.SendEmbed(context.Background(), "http://example.com/api/webhooks/1/2", TestEmbed(time.UTC)); err == nil {
		t.Fatal("expected error")
	}
}

func TestSuccessEmbedLayout(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	at := time.Date(2026, 10, 20, 16, 34, 0, 0, loc)
	ms := int64(2000)
	ok := SuccessEmbed(
		"AppData",
		"4",
		FormatDurationHHMMSS(&ms),
		FormatFiles(1875, 1875),
		FormatDataAmount(1084479242, 1084479242),
		at,
		loc,
	)
	if ok.Title != "✅ Backup erfolgreich abgeschlossen" {
		t.Fatalf("title: %s", ok.Title)
	}
	if ok.Color != ColorGreen {
		t.Fatal("color")
	}
	if len(ok.Fields) != 4 {
		t.Fatalf("fields: %d", len(ok.Fields))
	}
	if !ok.Fields[0].Inline || !ok.Fields[1].Inline || !ok.Fields[2].Inline || ok.Fields[3].Inline {
		t.Fatal("inline layout mismatch")
	}
	if ok.Fields[0].Name != "📦 Backup" || ok.Fields[1].Name != "⏱️ Dauer" || ok.Fields[2].Name != "📁 Dateien" || ok.Fields[3].Name != "💾 Datenmenge" {
		t.Fatalf("field names: %#v", ok.Fields)
	}
	if ok.Fields[1].Value != "00:00:02" {
		t.Fatalf("duration: %s", ok.Fields[1].Value)
	}
	if !strings.Contains(ok.Fields[3].Value, "GiB") {
		t.Fatalf("data amount: %s", ok.Fields[3].Value)
	}
	if ok.Footer == nil || ok.Footer.Text != "ZimaOS Backup • Task 4 • heute um 16:34 Uhr" {
		// "heute" depends on "now"; if test runs on another day, accept date form
		if ok.Footer == nil || !strings.Contains(ok.Footer.Text, "ZimaOS Backup • Task 4 •") {
			t.Fatalf("footer: %#v", ok.Footer)
		}
	}

	fail := FailureEmbed("AppData", "4", "00:00:01", "0 / 10", "0.00 / 1.00 MiB", "disk full", at, loc)
	if fail.Color != ColorRed || len(fail.Fields) < 5 {
		t.Fatal("failure embed")
	}
}

func TestMaskWebhook(t *testing.T) {
	masked := MaskWebhook("https://discord.com/api/webhooks/123/" + "secret" + "token")
	if strings.Contains(masked, "secret"+"token") {
		t.Fatalf("secret leaked: %s", masked)
	}
}

func TestFormatDurationAndData(t *testing.T) {
	ms := int64(3723000)
	if got := FormatDurationHHMMSS(&ms); got != "01:02:03" {
		t.Fatalf("got %s", got)
	}
	got := FormatDataAmount(1024*1024*1024, 1024*1024*1024)
	if got != "1.00 / 1.00 GiB" {
		t.Fatalf("got %s", got)
	}
}
