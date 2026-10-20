package composebackup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectSourceCandidatesScoresComposeDirs(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "good")
	_ = os.MkdirAll(filepath.Join(good, "a"), 0o755)
	_ = os.WriteFile(filepath.Join(good, "a", "compose.yml"), []byte("services: {}\n"), 0o644)

	cands := DetectSourceCandidates(good)
	if len(cands) == 0 {
		t.Fatal("expected candidates")
	}
	if cands[0].Path != filepath.Clean(good) || !cands[0].Recommended {
		t.Fatalf("first=%+v", cands[0])
	}
}

func TestIsUnder(t *testing.T) {
	root := filepath.Clean("/data")
	if !isUnder(filepath.Join(root, "x"), root) {
		t.Fatal("child should be under root")
	}
	if isUnder(filepath.Clean("/other"), root) {
		t.Fatal("sibling must not match")
	}
}

func TestValidateSourceDir(t *testing.T) {
	if err := ValidateSourceDir("", "/host/casaos/apps"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSourceDir("/host/casaos/apps", "/host/casaos/apps"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSourceDir("/etc/passwd", "/host/casaos/apps"); err == nil {
		t.Fatal("expected reject")
	}
}
