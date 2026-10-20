package auth

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/MarcelRuh/zimaos-backup-manager/internal/db"
)

func TestSetupLoginLogoutAndBruteForce(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	svc := NewService(database.SQL)
	ctx := context.Background()

	needs, err := svc.NeedsSetup(ctx)
	if err != nil || !needs {
		t.Fatalf("expected needs setup")
	}

	sess, err := svc.CompleteSetup(ctx, "admin", "password1234")
	if err != nil {
		t.Fatal(err)
	}
	if sess.CSRFToken == "" {
		t.Fatal("missing csrf")
	}

	_, err = svc.CompleteSetup(ctx, "admin2", "password1234")
	if err != ErrSetupCompleted {
		t.Fatalf("expected setup completed, got %v", err)
	}

	_, err = svc.Login(ctx, "admin", "wrong-password", "1.2.3.4")
	if err != ErrInvalidCredentials {
		t.Fatalf("expected invalid credentials")
	}

	loginSess, err := svc.Login(ctx, "admin", "password1234", "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	gotSess, user, err := svc.GetSession(ctx, loginSess.ID)
	if err != nil || user.Username != "admin" || gotSess.ID == "" {
		t.Fatalf("session lookup failed: %v", err)
	}

	if err := svc.Logout(ctx, loginSess.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.GetSession(ctx, loginSess.ID); err != ErrInvalidSession {
		t.Fatalf("expected invalid session after logout")
	}

	for i := 0; i < MaxFailedAttempts; i++ {
		_, _ = svc.Login(ctx, "admin", "bad", "9.9.9.9")
	}
	_, err = svc.Login(ctx, "admin", "password1234", "9.9.9.9")
	if err != ErrLockedOut {
		t.Fatalf("expected lockout, got %v", err)
	}
}
