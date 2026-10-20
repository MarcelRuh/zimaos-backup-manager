package secretbox

import (
	"path/filepath"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	box, err := OpenOrCreate(filepath.Join(t.TempDir(), "key"))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal("super-secret")
	if err != nil {
		t.Fatal(err)
	}
	if !IsSealed(sealed) {
		t.Fatalf("expected sealed prefix, got %q", sealed)
	}
	plain, err := box.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "super-secret" {
		t.Fatalf("got %q", plain)
	}
}

func TestOpenLegacyPlaintext(t *testing.T) {
	box, err := OpenOrCreate(filepath.Join(t.TempDir(), "key"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := box.Open("legacy-password")
	if err != nil {
		t.Fatal(err)
	}
	if plain != "legacy-password" {
		t.Fatalf("got %q", plain)
	}
}

func TestOpenOrCreateReusesKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	a, err := OpenOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := a.Seal("x")
	if err != nil {
		t.Fatal(err)
	}
	b, err := OpenOrCreate(path)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := b.Open(sealed)
	if err != nil || plain != "x" {
		t.Fatalf("reuse failed: %v %q", err, plain)
	}
}
