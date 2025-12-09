package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/engine"
	"github.com/okira-e/veriflow/tests/integration/helpers"
)

func TestVariableInjectables(t *testing.T) {
	server := helpers.SpinTestServer(map[string]http.HandlerFunc{
		"/users/register": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			defer r.Body.Close()

			var data struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			_ = json.Unmarshal(body, &data)

			w.WriteHeader(http.StatusCreated)
			resp := struct {
				ID    string `json:"id"`
				Token string `json:"token"`
				Data  struct {
					Email string `json:"email"`
				} `json:"data"`
			}{
				ID:    "u1",
				Token: "t1",
			}
			resp.Data.Email = data.Email

			b, _ := json.Marshal(resp)
			w.Write(b)
		},

		"/users/login": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			defer r.Body.Close()

			var data struct {
				Email string `json:"email"`
			}
			_ = json.Unmarshal(body, &data)

			// fail if unresolved placeholder appears
			if strings.Contains(data.Email, "{{var:email}}") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// fail if empty
			if data.Email == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// fail partial placeholder
			if strings.Contains(data.Email, "{{var:email") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// fail case sensitivity
			if strings.Contains(data.Email, "{{Var:email}}") {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.WriteHeader(http.StatusOK)
			resp := struct {
				Success bool   `json:"success"`
				Email   string `json:"email"`
			}{
				Success: true,
				Email:   data.Email,
			}

			b, _ := json.Marshal(resp)
			w.Write(b)
		},
	})
	defer server.Close()

	cfg, err := config.LoadConfig(helpers.TestDataPath("bindings/defined.json"))
	if err != nil {
		t.Fatalf("failed loading config: %v", err)
	}

	cfg.BaseUrl = server.URL

	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	cliopts.JSONOutput = true
	
	err = runner.Execute()
	if err != nil {
		t.Fatalf("flow execution failed: %v", err)
	}
}
