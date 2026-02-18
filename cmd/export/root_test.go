package export

import (
	"strings"
	"testing"

	"github.com/okira-e/veriflow/app"
	. "github.com/okira-e/veriflow/app/opt"
)

func TestConvertStepToCurl(t *testing.T) {
	baseUrl := "https://api.example.com"

	tests := []struct {
		name        string
		step        *app.Step
		assertions  []func(t *testing.T, out string)
		expectError bool
	}{
		{
			name: "basic GET request",
			step: &app.Step{
				Name: "get-user",
				Request: app.Request{
					Method: "GET",
					Path:   "/users/1",
				},
			},
			assertions: []func(t *testing.T, out string){
				assertContains("curl"),
				assertContains("GET"),
				assertContains(baseUrl + "/users/1"),
			},
		},
		{
			name: "POST with JSON body",
			step: &app.Step{
				Name: "create-user",
				Request: app.Request{
					Method: "POST",
					Path:   "/users",
					Json: Some[any](map[string]any{
						"name": "alice",
						"age":  30,
					}),
				},
			},
			assertions: []func(t *testing.T, out string){
				assertContains("POST"),
				assertContains("/users"),
				assertContains("application/json"),
				assertAnyOf(
					assertContains(`"name":"alice"`),
					assertContains(`"age":30`),
				),
			},
		},
		{
			name: "POST with XML body",
			step: &app.Step{
				Name: "xml-post",
				Request: app.Request{
					Method: "POST",
					Path:   "/xml",
					Xml:    Some(`<user><id>1</id></user>`),
				},
			},
			assertions: []func(t *testing.T, out string){
				assertContains("POST"),
				assertContains("/xml"),
				assertContains("application/xml"),
				assertContains("<user>"),
			},
		},
		{
			name: "disable headers removes content-type",
			step: &app.Step{
				Name: "no-headers",
				Request: app.Request{
					Method:         "POST",
					Path:           "/raw",
					Json:           Some[any](map[string]any{"x": "y"}),
					DisableHeaders: true,
				},
			},
			assertions: []func(t *testing.T, out string){
				assertNotContains("Content-Type"),
			},
		},
		{
			name: "timeout option is reflected",
			step: &app.Step{
				Name: "timeout",
				Request: app.Request{
					Method: "GET",
					Path:   "/slow",
				},
				Options: app.StepOptions{
					Timeout: Some("5s"),
				},
			},
			assertions: []func(t *testing.T, out string){
				assertAnyOf(
					assertContains("--max-time"),
					assertContains("5"),
				),
			},
		},
		{
			name: "empty method",
			step: &app.Step{
				Name: "default-method",
				Request: app.Request{
					Path: "/oops",
				},
			},
			assertions: []func(t *testing.T, out string){
				assertContains("-X 'GET'"),
			},
		},
		{
			name: "empty path",
			step: &app.Step{
				Name: "base-path",
				Request: app.Request{
					Method: "GET",
				},
			},
			assertions: []func(t *testing.T, out string){
				assertContains(baseUrl),
			},
		},
		{
			name: "single file upload",
			step: &app.Step{
				Name: "upload-file",
				Request: app.Request{
					Method: "POST",
					Path:   "/upload",
					Files:  Some(map[string]string{"document": "test-files/sample.pdf"}),
				},
			},
			assertions: []func(t *testing.T, out string){
				assertContains("POST"),
				assertContains("/upload"),
				assertContains("-F"),
				assertContains("document=@test-files/sample.pdf"),
			},
		},
		{
			name: "multiple file uploads",
			step: &app.Step{
				Name: "upload-multiple",
				Request: app.Request{
					Method: "POST",
					Path:   "/batch-upload",
					Files: Some(map[string]string{
						"avatar": "images/avatar.jpg",
						"resume": "docs/resume.pdf",
					}),
				},
			},
			assertions: []func(t *testing.T, out string){
				assertContains("POST"),
				assertContains("/batch-upload"),
				assertAnyOf(
					assertContains("avatar=@images/avatar.jpg"),
					assertContains("resume=@docs/resume.pdf"),
				),
			},
		},
		{
			name: "single custom header",
			step: &app.Step{
				Name: "with-auth",
				Request: app.Request{
					Method:  "GET",
					Path:    "/protected",
					Headers: Some(map[string]string{"Authorization": "Bearer token123"}),
				},
			},
			assertions: []func(t *testing.T, out string){
				assertContains("GET"),
				assertContains("/protected"),
				assertContains("-H"),
				assertContains("Authorization: Bearer token123"),
			},
		},
		{
			name: "multiple custom headers",
			step: &app.Step{
				Name: "with-headers",
				Request: app.Request{
					Method: "POST",
					Path:   "/api",
					Json:   Some[any](map[string]any{"data": "test"}),
					Headers: Some(map[string]string{
						"Authorization": "Bearer token",
						"X-API-Key":     "secret",
					}),
				},
			},
			assertions: []func(t *testing.T, out string){
				assertContains("POST"),
				assertContains("/api"),
				assertContains("application/json"),
				assertAnyOf(
					assertContains("Authorization: Bearer token"),
					assertContains("X-API-Key: secret"),
				),
			},
		},
		{
			name: "custom header overrides auto Content-Type",
			step: &app.Step{
				Name: "override-content-type",
				Request: app.Request{
					Method:  "POST",
					Path:    "/custom",
					Json:    Some[any](map[string]any{"data": "test"}),
					Headers: Some(map[string]string{"Content-Type": "text/plain"}),
				},
			},
			assertions: []func(t *testing.T, out string){
				assertContains("POST"),
				assertContains("/custom"),
				// Both headers should appear, but custom one comes after
				assertContains("Content-Type: application/json"),
				assertContains("Content-Type: text/plain"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := convertStepToCurl(tt.step, baseUrl)

			if tt.expectError {
				if err == nil {
					t.Fatalf("expected error, got none. output=%q", out)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if strings.TrimSpace(out) == "" {
				t.Fatal("empty curl output")
			}

			for _, assert := range tt.assertions {
				assert(t, out)
			}
		})
	}
}

/* ---------- assertion helpers ---------- */

func assertContains(substr string) func(*testing.T, string) {
	return func(t *testing.T, out string) {
		t.Helper()
		if !strings.Contains(out, substr) {
			t.Fatalf("expected output to contain %q\noutput:\n%s", substr, out)
		}
	}
}

func assertNotContains(substr string) func(*testing.T, string) {
	return func(t *testing.T, out string) {
		t.Helper()
		if strings.Contains(out, substr) {
			t.Fatalf("expected output NOT to contain %q\noutput:\n%s", substr, out)
		}
	}
}

func assertAnyOf(asserts ...func(*testing.T, string)) func(*testing.T, string) {
	return func(t *testing.T, out string) {
		t.Helper()
		for _, a := range asserts {
			tt := &testing.T{}
			a(tt, out)
			if !tt.Failed() {
				return
			}
		}
		t.Fatalf("none of the expected conditions matched\noutput:\n%s", out)
	}
}
