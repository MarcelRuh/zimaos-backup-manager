package store

import "testing"

func TestNormalizeComposeHook(t *testing.T) {
	cases := map[string]string{
		"":           ComposeHookOff,
		"off":        ComposeHookOff,
		"before":     ComposeHookBefore,
		"BEFORE":     ComposeHookBefore,
		"on_success": ComposeHookOnSuccess,
		"after":      ComposeHookOnSuccess,
		"both":       ComposeHookBoth,
		"garbage":    ComposeHookOff,
	}
	for in, want := range cases {
		if got := NormalizeComposeHook(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestComposeTimingHelpers(t *testing.T) {
	cases := []struct {
		hook   string
		before bool
		after  bool
	}{
		{ComposeHookOff, false, false},
		{ComposeHookBefore, true, false},
		{ComposeHookOnSuccess, false, true},
		{ComposeHookBoth, true, true},
	}
	for _, tc := range cases {
		p := TaskPrefs{ComposeBackupHook: tc.hook}
		if p.ComposeBefore() != tc.before || p.ComposeOnSuccess() != tc.after {
			t.Fatalf("%s: before=%v after=%v", tc.hook, p.ComposeBefore(), p.ComposeOnSuccess())
		}
	}
}
