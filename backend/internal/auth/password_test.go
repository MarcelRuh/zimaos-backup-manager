package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("super-secret-password")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := VerifyPassword(hash, "super-secret-password")
	if err != nil || !ok {
		t.Fatalf("expected verify ok, err=%v ok=%v", err, ok)
	}
	ok, err = VerifyPassword(hash, "wrong-password")
	if err != nil || ok {
		t.Fatalf("expected verify fail, err=%v ok=%v", err, ok)
	}
}
