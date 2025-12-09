package helpers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func SpinTestServer(routes map[string]http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()
	for p, h := range routes {
		mux.Handle(p, h)
	}
	return httptest.NewServer(mux)
}

func Log(t *testing.T, msg string, args ...any) {
	t.Helper()
	t.Logf(msg, args...)
}

func TestDataPath(p string) string {
	wd, _ := os.Getwd()

	// walk upward until we find go.mod
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "testdata", p)
		}
		next := filepath.Dir(dir)
		if next == dir {
			panic("go.mod not found")
		}
		dir = next
	}
}
