package helpers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/engine"
)

func SpinTestServer(routes map[string]http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()
	for p, h := range routes {
		mux.Handle(p, h)
	}
	return httptest.NewServer(mux)
}

func LoadConfigT(t *testing.T, path string) *config.Cfg {
	t.Helper()
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("failed loading %s: %v", path, err)
	}
	return cfg
}

func RunFlowT(t *testing.T, cfg *config.Cfg, serverURL string) error {
	t.Helper()

	cfg.BaseUrl = serverURL

	r := engine.NewRunner(engine.RunnerSettings{
		Cfg: cfg,
	})

	return r.Execute()
}
