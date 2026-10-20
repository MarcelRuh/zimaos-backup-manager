package composebackup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const targetComposeName = "docker-compose.yml"

func fileReadable(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func dirWritable(path string) bool {
	if path == "" {
		return false
	}
	st, err := os.Stat(path)
	if err != nil {
		// Parent may exist and be writable for MkdirAll.
		parent := filepath.Dir(path)
		return isWritableDir(parent)
	}
	if !st.IsDir() {
		return false
	}
	return isWritableDir(path)
}

func isWritableDir(dir string) bool {
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return false
	}
	test := filepath.Join(dir, ".zbm-write-test")
	if err := os.WriteFile(test, []byte("ok"), 0o600); err != nil {
		return false
	}
	_ = os.Remove(test)
	return true
}

func copyComposeIntoAppData(composeSrc, appDataDir string) (string, int64, error) {
	composeSrc = strings.TrimSpace(composeSrc)
	appDataDir = strings.TrimSpace(appDataDir)
	if composeSrc == "" || appDataDir == "" {
		return "", 0, fmt.Errorf("missing compose or appdata path")
	}
	if err := os.MkdirAll(appDataDir, 0o750); err != nil {
		return "", 0, err
	}
	dst := filepath.Join(appDataDir, targetComposeName)
	tmp := dst + ".partial"

	in, err := os.Open(composeSrc)
	if err != nil {
		return "", 0, err
	}
	defer in.Close()

	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", 0, err
	}
	n, copyErr := io.Copy(out, io.LimitReader(in, maxFileBytes))
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return "", 0, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", 0, closeErr
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	return dst, n, nil
}
