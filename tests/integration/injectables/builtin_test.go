package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/engine"
	"github.com/okira-e/veriflow/tests/integration/helpers"
)

func TestBuiltinInjectables(t *testing.T) {
	server := helpers.SpinTestServer(map[string]http.HandlerFunc{
		"/users/register": func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(400)
				return
			}
			defer r.Body.Close()

			var data struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			json.Unmarshal(body, &data)

			if strings.Contains(data.Email, "{{RUN_ID}}") {
				w.WriteHeader(http.StatusBadRequest)
				helpers.Log(t, "Found bare {{RUN_ID}} in payload body: %s", data.Email)
				return
			}

			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id":"u1","token":"t1"}`))
		},
	})
	defer server.Close()

	cfg, err := config.LoadConfig("../../../testdata/injectables/builtin.json")
	if err != nil {
		t.Fatalf("failed loading config path: %v", err)
	}

	cfg.BaseUrl = server.URL

	runner := engine.NewRunner(engine.RunnerSettings{
		Cfg: cfg,
	})

	err = runner.Execute()
	if err != nil {
		t.Fatalf("flow failed: %v", err)
	}
}
