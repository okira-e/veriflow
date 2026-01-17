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
			resp := struct {
				ID    string `json:"id"`
				Token string `json:"token"`
				Email string `json:"email"`
			}{
				ID:    "u1",
				Token: "t1",
				Email: data.Email, // Test that the assertion will succeed by injecting the {{RUN_ID}} in the assert block
			}

			b, _ := json.Marshal(resp)
			w.Write(b)
		},
	})
	defer server.Close()

	cfg, err := config.LoadConfig(helpers.TestDataPath("bindings/builtin.json"))
	if err != nil {
		t.Fatalf("failed loading config path: %v", err)
	}

	cfg.BaseUrl = server.URL

	runner := engine.NewRunner(engine.RunnerSettings{
		Cfg: cfg,
	})

	for _, flow := range cfg.Flows {
		for _, step := range flow.Steps {
			err = runner.Execute(step)
			if err != nil {
				t.Fatalf("step failed: %v", err)
			}
		}
	}
}
