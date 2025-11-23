package integration

import (
	"net/http"
	"testing"

	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/engine"
	testHelpers "github.com/okira-e/veriflow/tests/integration/helpers"
)

func TestUserOnboarding(t *testing.T) {
	server := testHelpers.SpinTestServer(map[string]http.HandlerFunc{
		"/users/register": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":"u1","token":"t1"}`))
		},
	})
	defer server.Close()

	cfg, err := config.LoadConfig("../../../testdata/flows/user_onboarding.json")
	if err != nil {
		t.Fatalf("failed loading config path: %v", err)
	}

	cfg.BaseUrl = server.URL

	r := engine.NewRunner(engine.RunnerSettings{
		Cfg: cfg,
	})

	err = r.Execute()
	if err != nil {
		t.Fatalf("flow failed: %v", err)
	}
}
