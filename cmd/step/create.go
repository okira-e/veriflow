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
	"github.com/okira-e/veriflow/app/opt"
	"github.com/okira-e/veriflow/app/utils"
	"github.com/spf13/cobra"
)

type createCmdFlags struct {
	Flow       string `validate:"required"`
	Method     string `validate:"required,oneof=GET POST PUT PATCH DELETE OPTIONS HEAD"`
	Path       string `validate:"required,startswith=/"`
	Json       string
	Status     int `validate:"required,gt=99,lt=600"`
	AssertExpr []string
}

func newCreateCmd() *cobra.Command {
	var flags createCmdFlags

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new test step",
		Run: func(cmd *cobra.Command, args []string) {
			err := runCreateCmd(cmd, args, flags)
			utils.HandleCliError(err)
		},
	}

	cmd.Flags().StringVar(&flags.Flow, "flow", "", "Flow this step belongs to")
	cmd.Flags().StringVar(&flags.Method, "method", "", "HTTP method")
	cmd.Flags().StringVar(&flags.Path, "path", "", "Request path")
	cmd.Flags().StringVar(&flags.Json, "json", "", "JSON body (optional)")
	cmd.Flags().IntVar(&flags.Status, "status", 0, "Asserted HTTP status code")
	cmd.Flags().StringArrayVar(&flags.AssertExpr, "assert", []string{}, "Asserted result body")

	return cmd
}

func runCreateCmd(cmd *cobra.Command, args []string, flags createCmdFlags) error {
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

	cfg, err := config.LoadConfig()
	if err != nil {
		return oops.Err(oops.Internal, "failed to load config", err)
	}

	// Make sure there's at least one flow to add a step to.
	if len(cfg.Flows) == 0 {
		return oops.Err(oops.EmptyFlows, "you need at least one flow to add a step to. Create a flow with `veriflow flow create [name]`", nil)
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

	if err := utils.ValidateStruct(&flags); err != nil {
		return oops.Err(oops.MissingRequiredFlag, "missing required flags", err)
	}

	// Validate the flow provided exists.
	flow, ok := cfg.GetFlow(flags.Flow)
	if !ok {
		return oops.Err(oops.FlowDoesntExist, "flow provided doesn't exist", nil)
	}

	step, err := buildStepFromFlags(stepName, &flags)
	if err != nil {
		return oops.Err(oops.Internal, "failed to build step from flags", err)
	}

	appErr := flow.AddStep(step)
	if appErr != nil {
		return appErr
	}

	err = cfg.UpdateFlow(flow)
	if err != nil {
		return oops.Err(oops.Internal, "failed to update the config", err)
	}

	return nil
}

func promptForRequiredFlags(flags *createCmdFlags) error {
	if flags.Flow == "" {
		cfg, err := config.LoadConfig()
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

func promptForOptionalFlags(flags *createCmdFlags) error {
	var err error

	if flags.Json == "" {
		flags.Json, err = cli.PromptForJson("JSON to send", "", false)
		if err != nil {
			return err
		}
	}

	// [OKI-36] do this bit for asserting through the CLI.
	// if flags.AssertExpr == "" {
	// 	assertExpr, err := cli.PromptForString("Assertion expression\nhi", "", false)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	flags.AssertExpr = assertExpr
	// }

	return nil
}

func buildStepFromFlags(stepName string, flags *createCmdFlags) (*app.Step, error) {
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
	if len(flags.AssertExpr) != 0 {
		// Build the assertions like: equals, contain, etc from the CLI expression.
		all, err := buildAssertObjectFromExpressions(flags.AssertExpr)
		if err != nil {
			return nil, err
		}

		assert = app.NewAssert(flags.Status, opt.Some(all))
	} else {
		// Nothing to assert other than the status of the response.
		assert = app.NewAssert(flags.Status, opt.None[[]app.Assertion]())
	}

	request := app.NewRequest(flags.Method, flags.Path, parsedJson)

	exports := app.Exports{}

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
func buildAssertObjectFromExpressions(assertExpr []string) ([]app.Assertion, error) {
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

	asserts := make([]app.Assertion, 0, len(assertExpr))

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
			asserts = append(asserts, app.NewAssertion(path, true, false, false, false, ""))
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
				asserts = append(asserts, app.NewAssertion(path, false, false, true, false, val))
			case "contains":
				asserts = append(asserts, app.NewAssertion(path, false, true, false, false, val))
			default:
				// Should never happen due to regex, but keep a guard.
				return nil, fmt.Errorf("assertion #%d: unsupported type %q", i+1, kind)
			}
			continue
		}

		return nil, fmt.Errorf(
			"invalid assertion syntax at #%d: %q. Expected one of:\n  - exists <jsonpath>\n  - equals <jsonpath> <value>\n  - contains <jsonpath> <value>",
			i+1, raw,
		)
	}

	return asserts, nil
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
