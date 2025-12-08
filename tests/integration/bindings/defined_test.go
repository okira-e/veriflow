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

func TestVariableInjectables(t *testing.T) {
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
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(400)
				return
			}
			defer r.Body.Close()

			var data struct {
				Email string `json:"email"`
			}
			json.Unmarshal(body, &data)

			if strings.Contains(data.Email, "{{var:email}}") {
				w.WriteHeader(http.StatusBadRequest)
				helpers.Log(t, "Found bare {{var:email}} in payload body: %s", data.Email)
				return
			}

			if data.Email == "" {
				w.WriteHeader(http.StatusBadRequest)
				helpers.Log(t, "Email was not injected: %s", data.Email)
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

	cfg, err := config.LoadConfig("../../../testdata/bindings/defined.json")
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
