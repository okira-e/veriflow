package step

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/cli"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/oops"
	. "github.com/okira-e/veriflow/app/opt"
	"github.com/okira-e/veriflow/app/utils"
	"github.com/spf13/cobra"
)

type addCmdFlags struct {
	Flow              string `validate:"required"`
	Method            string `validate:"required,oneof=GET POST PUT PATCH DELETE OPTIONS HEAD"`
	Path              string `validate:"required,startswith=/"`
	Json              string
	Status            int `validate:"required,gt=99,lt=600"`
	NoSave bool
	AssertExpressions []string
	ExportExpressions []string
}

func newAddCmd() *cobra.Command {
	var flags addCmdFlags

	cmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Add a new test step",
		Long: `
Assertions syntax:
  exists <jsonpath>
    Example: --assert "exists $.data.token"

  equals <jsonpath> <value>
    Example: --assert "equals $.user.id 42"

  contains <jsonpath> <value>
    Example: --assert "contains $.roles admin"

Exports syntax:
  <varname> <jsonpath>
    Example: --export "user_id $.data.user_id"
    Example: --export "token $.data.token"
`,

		Run: func(cmd *cobra.Command, args []string) {
			err := runAddCmd(cmd, args, flags)
			cli.HandleCliError(err, cliopts.Verbose)
		},
	}

	cmd.Flags().StringVar(&flags.Flow, "flow", "", "Flow this step belongs to")
	cmd.Flags().StringVar(&flags.Method, "method", "", "HTTP method")
	cmd.Flags().StringVar(&flags.Path, "path", "", "Request path")
	cmd.Flags().StringVar(&flags.Json, "json", "", "JSON body (optional)")
	cmd.Flags().IntVar(&flags.Status, "status", 0, "Asserted HTTP status code")
	cmd.Flags().StringArrayVar(&flags.AssertExpressions, "assert", []string{}, "Asserted result body")
	cmd.Flags().StringArrayVar(&flags.ExportExpressions, "export", []string{}, "Export variable from response (format: varname jsonpath)")
	cmd.Flags().BoolVar(&flags.NoSave, "no-save", false, "Modify the config but don't save on disk")

	return cmd
}

func runAddCmd(cmd *cobra.Command, args []string, flags addCmdFlags) error {
	var stepName string

	if len(args) > 0 {
		stepName = args[0]
	}

	if stepName == "" {
		var err error
		stepName, err = cli.PromptForString("Step name", "register", true)
		if err != nil {
			return oops.Err(oops.PromptError, "failed to prompt for a step name", err)
		}
	}

	cfg, err := config.LoadConfig(cliopts.ConfigFile)
	if err != nil {
		return oops.Err(oops.Internal, "failed to load config", err)
	}

	// Make sure there's at least one flow to add a step to.
	if len(cfg.Flows) == 0 {
		return oops.Err(oops.EmptyFlows, "you need at least one flow to add a step to. Add a flow with `veriflow flow add [name]`", nil)
	}

	// Only prompt for optional parameters if --no-interactive is not provided.
	// Otherwise, assume the bot provided everything it needs and nothing more.
	if !cliopts.NonInteractive {
		if err := promptForRequiredFlags(&flags); err != nil {
			return oops.Err(oops.ValidationError, "flags validation error", err)
		}

		if err := promptForOptionalFlags(&flags); err != nil {
			return oops.Err(oops.Internal, "failed to validate optional flags", err)
		}
	}

	if err := cli.ValidateStruct(&flags); err != nil {
		return oops.Err(oops.MissingRequiredFlag, "missing required flags", err)
	}

	// Validate the flow provided exists.
	flow, ok := cfg.GetFlow(flags.Flow)
	if !ok {
		return oops.Err(oops.FlowNotFound, "flow provided doesn't exist", nil)
	}

	step, err := buildStepFromFlags(stepName, &flags)
	if err != nil {
		return err
	}

	appErr := flow.AddStep(step)
	if appErr != nil {
		return appErr
	}

	err = cfg.UpdateFlow(flow)
	if err != nil {
		return oops.Err(oops.Internal, "failed to update the config", err)
	}

	if !flags.NoSave {
		if err = cfg.Save(); err != nil {
			return err
		}
	}

	if !cliopts.Silent {
		msg := fmt.Sprintf("Step \"%s\" has been added to the \"%s\" flow", stepName, flow.Name)
		utils.PrintInColor("green", msg, true)
	}

	return nil
}

// promptForRequiredFlags sets the value in the flags object from cli prompting
func promptForRequiredFlags(flags *addCmdFlags) error {
	if flags.Flow == "" {
		cfg, err := config.LoadConfig(cliopts.ConfigFile)
		if err != nil {
			return err
		}

		flowNames := make([]string, len(cfg.Flows))
		for i, flowPtr := range cfg.Flows {
			flowNames[i] = flowPtr.Name
		}

		options := huh.NewOptions(flowNames...)
		flags.Flow, err = cli.PromptForOption("Which flow does this belong to?", options, true)
		if err != nil {
			return err
		}
	}

	if flags.Method == "" {
		var err error
		flags.Method, err = cli.PromptForString("Method", "POST", true)
		if err != nil {
			return err
		}
	}

	if flags.Path == "" {
		var err error
		flags.Path, err = cli.PromptForString("Path", "/users/register", true)
		if err != nil {
			return err
		}
	}

	if flags.Status == 0 {
		var err error
		flags.Status, err = cli.PromptForInt("Status to assert", "201", true)
		if err != nil {
			return err
		}
	}

	return nil
}

func promptForOptionalFlags(flags *addCmdFlags) error {
	var err error

	if flags.Json == "" && flags.Method != "GET" {
		flags.Json, err = cli.PromptForJson("JSON to send", "", false)
		if err != nil {
			return err
		}
	}

	if len(flags.AssertExpressions) == 0 {
		if err := promptForAssertions(flags); err != nil {
			return err
		}
	}

	if len(flags.ExportExpressions) == 0 {
		if err := promptForExports(flags); err != nil {
			return err
		}
	}

	return nil
}

func promptForAssertions(flags *addCmdFlags) error {
	addAssertions, err := cli.PromptForBool("Add assertions to validate the response body?")
	if err != nil {
		return err
	}

	if !addAssertions {
		return nil
	}

	assertions := []string{}

	// @TODO: Fix saying "contains null" setting the value as "null" instead of JSON's null.
	for {
		// Prompt for assertion type
		assertionTypeOptions := huh.NewOptions("exists", "equals", "contains")
		assertionType, err := cli.PromptForOption("Assertion type (exists/equals/contains)", assertionTypeOptions, true)
		if err != nil {
			return err
		}

		// Prompt for JSONPath
		jsonPath, err := cli.PromptForString("JSONPath (e.g., $.data.id)", "$.data.id", true)
		if err != nil {
			return err
		}

		// Ensure JSONPath starts with $
		jsonPath = strings.TrimSpace(jsonPath)
		if jsonPath != "" && jsonPath[0] != '$' {
			jsonPath = "$" + jsonPath
		}

		// Validate JSONPath format
		if err := validateJSONPath(jsonPath); err != nil {
			return oops.Err(oops.JSONPathValidationError, fmt.Sprintf("invalid JSONPath %q", jsonPath), err)
		}

		// Build assertion expression based on type
		var assertionExpr string
		switch assertionType {
		case "exists":
			assertionExpr = fmt.Sprintf("exists %s", jsonPath)
		case "equals":
			value, err := cli.PromptForString(fmt.Sprintf("Expected value for %s", jsonPath), "", true)
			if err != nil {
				return err
			}
			assertionExpr = fmt.Sprintf("equals %s %s", jsonPath, value)
		case "contains":
			value, err := cli.PromptForString(fmt.Sprintf("Substring to check for in %s", jsonPath), "", true)
			if err != nil {
				return err
			}
			assertionExpr = fmt.Sprintf("contains %s %s", jsonPath, value)
		default:
			return oops.Err(oops.ErrInvalidInput, fmt.Sprintf("unknown assertion type: %s", assertionType), nil)
		}

		assertions = append(assertions, assertionExpr)

		// Ask if they want to add another assertion
		addMore, err := cli.PromptForBool("Add another assertion?")
		if err != nil {
			return err
		}

		if !addMore {
			break
		}
	}

	flags.AssertExpressions = assertions
	return nil
}

func promptForExports(flags *addCmdFlags) error {
	addExports, err := cli.PromptForBool("Export variables from the response?")
	if err != nil {
		return err
	}

	if !addExports {
		return nil
	}

	exports := []string{}

	for {
		varName, err := cli.PromptForString("Variable name (e.g., user_id)", "", true)
		if err != nil {
			return err
		}

		jsonPath, err := cli.PromptForString("JSONPath (e.g., $.data.id)", "$.data.id", true)
		if err != nil {
			return err
		}

		jsonPath = strings.TrimSpace(jsonPath)
		if jsonPath != "" && jsonPath[0] != '$' {
			jsonPath = "$" + jsonPath
		}

		if err := validateJSONPath(jsonPath); err != nil {
			return oops.Err(oops.JSONPathValidationError, fmt.Sprintf("invalid JSONPath %q", jsonPath), err)
		}

		exportExpr := fmt.Sprintf("%s %s", strings.TrimSpace(varName), jsonPath)
		exports = append(exports, exportExpr)

		addMore, err := cli.PromptForBool("Add another export?")
		if err != nil {
			return err
		}

		if !addMore {
			break
		}
	}

	flags.ExportExpressions = exports
	return nil
}

func buildStepFromFlags(stepName string, flags *addCmdFlags) (*app.Step, error) {
	var parsedJson map[string]any = nil
	if flags.Json != "" {
		if err := json.Unmarshal([]byte(flags.Json), &parsedJson); err != nil {
			return nil, oops.Err(
				oops.ErrInvalidInput,
				"failed to parse the json passed for the request",
				err,
			)
		}
	}

	var assert app.Assert
	if len(flags.AssertExpressions) != 0 {
		// Build the assertions like: equals, contain, etc from the CLI expression.
		all, err := buildAssertObjectFromExpressions(flags.AssertExpressions)
		if err != nil {
			return nil, oops.Err(oops.AssertionExpressionParsingFailure, "failed to parse the assertion expression", err)
		}

		assert = app.NewAssert(flags.Status, Some(all))
	} else {
		// Nothing to assert other than the status of the response.
		assert = app.NewAssert(flags.Status, None[[]*app.Assertion]())
	}

	request := app.NewRequest(flags.Method, flags.Path, parsedJson)

	exports, err := buildExportsFromExpressions(flags.ExportExpressions)
	if err != nil {
		return nil, oops.Err(oops.ErrInvalidInput, "failed to parse export expressions", err)
	}

	step := app.NewStep(stepName, request, assert, exports)

	return step, nil
}

// buildAssertObjectFromExpressions converts CLI --assert expressions into []app.Assertion.
// Valid forms:
//
//	exists   <jsonpath>
//	equals   <jsonpath> <value>
//	contains <jsonpath> <value>
//
// @AI
func buildAssertObjectFromExpressions(assertExpr []string) ([]*app.Assertion, error) {
	// Patterns (case-insensitive). Uses RE2 via Go's regexp package.
	var (
		reExists = regexp.MustCompile(`(?i)^\s*exists\s+(\$[^\s]+)\s*$`)
		// equals/contains with value that can be:
		//  - "double quoted"
		//  - 'single quoted'
		//  - or unquoted (read until end, then trim whitespace)
		reWithVal = regexp.MustCompile(
			`(?i)^\s*(equals|contains)\s+(\$[^\s]+)\s+(?:(?:"([^"]*)")|(?:'([^']*)')|(.+))\s*$`,
		)
	)

	asserts := make([]*app.Assertion, 0, len(assertExpr))

	for i, raw := range assertExpr {
		s := strings.TrimSpace(raw)
		if s == "" {
			return nil, fmt.Errorf("assertion #%d is empty", i+1)
		}

		// exists
		if m := reExists.FindStringSubmatch(s); m != nil {
			path := m[1]
			if err := validateJSONPath(path); err != nil {
				return nil, fmt.Errorf("assertion #%d: %w", i+1, err)
			}
			assertion := app.Assertion{
				JsonPath: path,
				Exists:   Some(true),
				Contains: None[string](),
				Equals:   None[string](),
				Secret:   false,
			}
			asserts = append(asserts, &assertion)
			continue
		}

		// equals / contains
		if m := reWithVal.FindStringSubmatch(s); m != nil {
			kind := strings.ToLower(m[1])
			path := m[2]
			val := firstNonEmpty(m[3], m[4], strings.TrimSpace(m[5]))

			if err := validateJSONPath(path); err != nil {
				return nil, fmt.Errorf("assertion #%d: %w", i+1, err)
			}
			if val == "" {
				return nil, fmt.Errorf("assertion #%d: missing VALUE for %q", i+1, kind)
			}

			switch kind {
			case "equals":
				assertion := app.Assertion{
					JsonPath: path,
					Exists:   Some(true),
					Contains: None[string](),
					Equals:   Some(val),
					Secret:   false,
				}
				asserts = append(asserts, &assertion)
			case "contains":
				assertion := app.Assertion{
					JsonPath: path,
					Exists:   Some(true),
					Contains: Some(val),
					Equals:   None[string](),
					Secret:   false,
				}
				asserts = append(asserts, &assertion)
			default:
				// Should never happen due to regex, but keep a guard.
				return nil, fmt.Errorf("assertion #%d: unsupported type %q", i+1, kind)
			}
			continue
		}

		return nil, fmt.Errorf(
			"invalid assertion syntax at #%d: %q\n. Expected one of:\n  - exists <jsonpath>\n  - equals <jsonpath> <value>\n  - contains <jsonpath> <value>",
			i+1, raw,
		)
	}

	return asserts, nil
}

// buildExportsFromExpressions converts CLI --export expressions into app.Exports.
// Valid form: "varname jsonpath"
// Example: "user_id $.data.user_id"
func buildExportsFromExpressions(exportExpr []string) (app.Exports, error) {
	exports := app.Exports{}

	for i, raw := range exportExpr {
		s := strings.TrimSpace(raw)
		if s == "" {
			return nil, oops.Err(oops.ErrInvalidInput, fmt.Sprintf("export #%d is empty", i+1), nil)
		}

		// Split by whitespace - first part is varname, rest is jsonpath
		parts := strings.Fields(s)
		if len(parts) < 2 {
			return nil, oops.Err(
				oops.ErrInvalidInput,
				fmt.Sprintf("export #%d: invalid format. Expected 'varname jsonpath', got %q", i+1, raw),
				nil,
			)
		}

		varName := parts[0]
		jsonPath := strings.Join(parts[1:], " ")

		// Ensure JSONPath starts with $
		jsonPath = strings.TrimSpace(jsonPath)
		if jsonPath != "" && jsonPath[0] != '$' {
			jsonPath = "$" + jsonPath
		}

		// Validate JSONPath format
		if err := validateJSONPath(jsonPath); err != nil {
			return nil, oops.Err(oops.JSONPathValidationError, fmt.Sprintf("export #%d: invalid JSONPath", i+1), err)
		}

		// Validate variable name (should be a valid identifier)
		if varName == "" {
			return nil, oops.Err(oops.ErrInvalidInput, fmt.Sprintf("export #%d: variable name cannot be empty", i+1), nil)
		}

		exports[varName] = jsonPath
	}

	return exports, nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// Minimal JSONPath sanity check.
// We only enforce it starts with '$' and has no spaces; full JSONPath validation
// is left to the executor that actually applies the path.
func validateJSONPath(p string) error {
	if p == "" || p[0] != '$' {
		return fmt.Errorf("JSONPath must start with '$': %q", p)
	}
	if strings.ContainsAny(p, " \t\r\n") {
		return fmt.Errorf("JSONPath must not contain whitespace: %q", p)
	}
	return nil
}
