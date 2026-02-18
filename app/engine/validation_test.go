package engine

import (
	"reflect"
	"testing"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/config"
	. "github.com/okira-e/veriflow/app/opt"
)

func TestWalkJSON(t *testing.T) {
	fn := func(s string) (any, error) { return s + "_x", nil }

	t.Run("string", func(t *testing.T) {
		input := "hello"
		out, _ := walkJSON(input, fn)

		if out != "hello_x" {
			t.Fatalf("expected hello_x, got %v", out)
		}
	})

	t.Run("map", func(t *testing.T) {
		input := map[string]any{
			"a": "one",
			"b": 42,
		}

		out, _ := walkJSON(input, fn)

		expected := map[string]any{
			"a": "one_x",
			"b": 42,
		}

		if !reflect.DeepEqual(out, expected) {
			t.Fatalf("expected %v, got %v", expected, out)
		}
	})

	t.Run("slice", func(t *testing.T) {
		input := []any{"one", 2, "three"}

		out, _ := walkJSON(input, fn)

		expected := []any{"one_x", 2, "three_x"}

		if !reflect.DeepEqual(out, expected) {
			t.Fatalf("expected %v, got %v", expected, out)
		}
	})

	t.Run("nested", func(t *testing.T) {
		input := map[string]any{
			"a": []any{
				"one",
				map[string]any{
					"b": "two",
				},
			},
		}

		out, _ := walkJSON(input, fn)

		expected := map[string]any{
			"a": []any{
				"one_x",
				map[string]any{
					"b": "two_x",
				},
			},
		}

		if !reflect.DeepEqual(out, expected) {
			t.Fatalf("expected %v, got %v", expected, out)
		}
	})

	t.Run("no mutation", func(t *testing.T) {
		input := map[string]any{
			"a": "one",
		}

		_, _ = walkJSON(input, fn)

		if input["a"] != "one" {
			t.Fatalf("walkJSON should not mutate input")
		}
	})
}

func TestRunner_ConfigValidation(t *testing.T) {
	t.Run("undefined in path", func(t *testing.T) {
		cfg := &config.Cfg{BaseUrl: "", Flows: []*app.Flow{
			{
				Name: "test-flow",
				Steps: []*app.Step{
					{
						Name: "undefined-binding",
						Request: app.Request{
							Method: "GET",
							Path:   "{{bind:undefined_in_path}}",
							Json: Some[any](map[string]any{
								"id": "{{bind:undefined_id}}@example.com",
							}),
						},
					},
				},
			},
		}}

		runner := NewRunner(RunnerSettings{Cfg: cfg})

		err := runner.ValidateConfig()
		if err == nil {
			t.Fatalf("expected a validation error, got nil")
		}
	})

	t.Run("undefined in json", func(t *testing.T) {
		cfg := &config.Cfg{BaseUrl: "", Flows: []*app.Flow{
			{
				Name: "test-flow",
				Steps: []*app.Step{
					{
						Name: "undefined-binding",
						Request: app.Request{
							Method: "GET",
							Json: Some[any](map[string]any{
								"id": "{{bind:undefined_binding_in_json}}",
							}),
						},
					},
				},
			},
		}}

		runner := NewRunner(RunnerSettings{Cfg: cfg})

		err := runner.ValidateConfig()
		if err == nil {
			t.Fatalf("expected a validation error, got nil")
		}
	})
}
