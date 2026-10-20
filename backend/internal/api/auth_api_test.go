package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/api"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/app"
	"github.com/MarcelRuh/zimaos-backup-manager/internal/config"
)

func TestSetupLoginMeFlow(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		ListenAddr:        ":0",
		DataDir:           dir,
		DatabasePath:      filepath.Join(dir, "test.db"),
		ZimaOSAPIBaseURL:  "http://127.0.0.1:1",
		Timezone:          "Europe/Berlin",
		PollInterval:      config.DefaultPollInterval,
		SessionCookieName: "zbm_session",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	application, err := app.New(cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(application.Close)

	srv := httptest.NewServer(api.NewRouter(application))
	t.Cleanup(srv.Close)
	client := srv.Client()

	resp, err := client.Get(srv.URL + "/api/setup/status")
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		Data struct {
			NeedsSetup bool `json:"needs_setup"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&env)
	_ = resp.Body.Close()
	if !env.Data.NeedsSetup {
		t.Fatal("expected setup required")
	}

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "password1234"})
	resp, err = client.Post(srv.URL+"/api/setup", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("setup status %d: %s", resp.StatusCode, b)
	}
	var setupEnv struct {
		Data struct {
			CSRF string `json:"csrf_token"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&setupEnv)
	cookies := resp.Cookies()
	_ = resp.Body.Close()
	if setupEnv.Data.CSRF == "" || len(cookies) == 0 {
		t.Fatal("missing session/csrf")
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/auth/me", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("me status %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// Protected endpoint without CSRF must fail on POST
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/auth/logout", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected csrf forbid, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/api/auth/logout", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set("X-CSRF-Token", setupEnv.Data.CSRF)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout status %d", resp.StatusCode)
	}
	_ = resp.Body.Close()
	_ = os.Stdout
}
