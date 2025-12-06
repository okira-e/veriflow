package integration

import (
	"net/http"
	"testing"

	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/engine"
	"github.com/okira-e/veriflow/tests/integration/helpers"
)

func TestSmoke(t *testing.T) {
	server := helpers.SpinTestServer(map[string]http.HandlerFunc{
		"/users/register": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":"123","token":"test number 1"}`))
		},
	})
	defer server.Close()

	cfg, err := config.LoadConfig("../../../testdata/flows/smoke.json")
	if err != nil {
		t.Fatalf("failed loading config path: %v", err)
	}

	cfg.BaseUrl = server.URL

	runner := engine.NewRunner(engine.RunnerSettings{
		Cfg: cfg,
	})

	// @TODO: Make global args passable for testing
	err = runner.Execute()
	if err != nil {
		t.Fatalf("flow failed: %v", err)
	}
}
