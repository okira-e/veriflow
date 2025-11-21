//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	helpers "github.com/okira-e/veriflow/tests/e2e/helpers"
)

func Test_UserOnboarding(t *testing.T) {
	t.Parallel()

	// Routes specific to this flow
	server := helpers.NewServer(
		helpers.Route{
			Match: helpers.MatchExact(http.MethodPost, "/users/register"),
			Handler: func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any

				_ = json.NewDecoder(r.Body).Decode(&body)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				w.WriteHeader(http.StatusOK)

				_ = json.NewEncoder(w).Encode(map[string]any{"id": "u_123", "email": body["email"]})
			},
		},
		helpers.Route{
			Match: helpers.MatchExact(http.MethodPost, "/users/login"),
			Handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				_ = json.NewEncoder(w).Encode(map[string]string{"token": "header.payload.sig"})
			},
		},
	)
	defer server.Close()

	ctx, cancel := helpers.TestCtx()
	defer cancel()

	res := helpers.RunCLI(ctx,
		[]string{
			// "RUN_ID=42",
		},
		"run",
		"--config", filepath.Join("..", "..", "testdata", "flows", "user_onboarding.json"),
		"--base-url", server.URL,
		"--json-output",
	)

	if res.Err != nil {
		t.Fatalf("cli failed: %s\nstderr:\n%s", res.Err, res.Stderr)
	}
	if !strings.Contains(res.Stdout, "user-onboarding") {
		t.Fatalf("expected flow name in stdout; got:\n%s", res.Stdout)
	}
}
