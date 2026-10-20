package zimaos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestListAndStartTasks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/backup/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "tok" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []Task{{
				ID: "4", Name: "AppData", Status: "completed", LastBackupTime: 100,
				Source:   Device{Name: "Zima", Path: "/media/src", IsConnected: true},
				Subtasks: []Subtask{{ID: "4", Status: "completed", Progress: 100, Target: Device{Name: "Zima", Path: "/media/dst", IsConnected: true}}},
			}},
		})
	})
	mux.HandleFunc("/v2/backup/tasks/4/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "task start completed successfully: 1/1 subtasks started",
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "tok")
	tasks, err := c.ListTasks(context.Background())
	if err != nil || len(tasks) != 1 || tasks[0].ID != "4" {
		t.Fatalf("list failed: %v %#v", err, tasks)
	}
	msg, err := c.StartTask(context.Background(), "4")
	if err != nil || msg == "" {
		t.Fatalf("start failed: %v %s", err, msg)
	}
}

func TestLoginAndRetryOn401(t *testing.T) {
	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/users/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"token": map[string]any{
					"access_token": "fresh-token",
				},
			},
		})
	})
	mux.HandleFunc("/v2/backup/tasks", func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			http.Error(w, `{"message":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Authorization") != "fresh-token" {
			http.Error(w, "bad token", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []Task{}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "stale")
	c.Configure(srv.URL, "stale", "admin", "secret")
	var saved string
	c.SetTokenPersister(func(ctx context.Context, token string) error {
		saved = token
		return nil
	})
	tasks, err := c.ListTasks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected empty, got %#v", tasks)
	}
	if saved != "fresh-token" {
		t.Fatalf("expected persisted token, got %q", saved)
	}
}

func TestBearerPrefixStripped(t *testing.T) {
	if got := normalizeToken("Bearer abc.def"); got != "abc.def" {
		t.Fatalf("got %q", got)
	}
}

func TestUnavailableAPI(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", "")
	_, err := c.ListTasks(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEmptyTaskListAndBadJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/backup/tasks", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := NewClient(srv.URL, "")
	tasks, err := c.ListTasks(context.Background())
	if err != nil || len(tasks) != 0 {
		t.Fatalf("expected empty list: %v %#v", err, tasks)
	}

	mux2 := http.NewServeMux()
	mux2.HandleFunc("/v2/backup/tasks", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not-json`))
	})
	srv2 := httptest.NewServer(mux2)
	t.Cleanup(srv2.Close)
	c2 := NewClient(srv2.URL, "")
	if _, err := c2.ListTasks(context.Background()); err == nil {
		t.Fatal("expected bad response error")
	}
}

func TestUnknownStatusHelpers(t *testing.T) {
	if !IsRunningStatus("IN_PROGRESS") {
		t.Fatal("running")
	}
	if !IsCompletedStatus("Success") {
		t.Fatal("completed")
	}
	if !IsFailedStatus("ERROR") {
		t.Fatal("failed")
	}
	task := Task{
		Status: "completed",
		Subtasks: []Subtask{
			{Status: "completed", TotalFiles: 10, XferredFiles: 10, TotalSize: 100, XferredSize: 100, Target: Device{IsConnected: true}},
			{Status: "completed", TotalFiles: 5, XferredFiles: 5, TotalSize: 50, XferredSize: 50, Target: Device{IsConnected: false}},
		},
	}
	tf, xf := task.AggregateFiles()
	if tf != 15 || xf != 15 {
		t.Fatalf("files %d %d", tf, xf)
	}
	if !task.AnyTargetDisconnected() {
		t.Fatal("expected disconnected target")
	}
}
