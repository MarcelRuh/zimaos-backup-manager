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
	if err := c.SendEmbed(context.Background(), url, TestEmbed()); err != nil {
		t.Fatal(err)
	}
	if attempts < 2 {
		t.Fatalf("expected retry, attempts=%d", attempts)
	}
}

func TestInvalidWebhookRejected(t *testing.T) {
	c := NewClient()
	if err := c.SendEmbed(context.Background(), "http://example.com/api/webhooks/1/2", TestEmbed()); err == nil {
		t.Fatal("expected error")
	}
}

func TestMaskAndEmbeds(t *testing.T) {
	masked := MaskWebhook("https://discord.com/api/webhooks/123/secrettoken")
	if strings.Contains(masked, "secrettoken") {
		t.Fatalf("secret leaked: %s", masked)
	}
	ok := SuccessEmbed("AppData", "1m 2s", "10 / 10", "1 MiB / 1 MiB", time.Now())
	if ok.Color != ColorGreen {
		t.Fatal("color")
	}
	fail := FailureEmbed("AppData", "1m", "0 / 10", "0 / 1", "disk full", time.Now())
	if fail.Color != ColorRed || len(fail.Fields) < 5 {
		t.Fatal("failure embed")
	}
}
