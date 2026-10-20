package auth

import "testing"

func TestClientIPIgnoresXFFByDefault(t *testing.T) {
	got := ClientIP("10.0.0.5:1234", "1.2.3.4", false)
	if got != "10.0.0.5" {
		t.Fatalf("got %q", got)
	}
}

func TestClientIPHonorsXFFWhenTrusted(t *testing.T) {
	got := ClientIP("10.0.0.5:1234", "1.2.3.4, 10.0.0.5", true)
	if got != "1.2.3.4" {
		t.Fatalf("got %q", got)
	}
}

func TestIsTrustedProxy(t *testing.T) {
	if !IsTrustedProxy("127.0.0.1:8080", []string{"127.0.0.1"}) {
		t.Fatal("expected trusted")
	}
	if IsTrustedProxy("10.0.0.1:1", []string{"127.0.0.1"}) {
		t.Fatal("expected untrusted")
	}
}
