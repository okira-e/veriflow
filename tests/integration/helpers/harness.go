package helpers

import (
	"net/http"
	"net/http/httptest"
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
