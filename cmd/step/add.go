// Copyright 2026 Omar Khaleel
// Licensed under the Apache License, Version 2.0.
// See LICENSE file for details.

package step

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/cli"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/logging"
	"github.com/okira-e/veriflow/app/oops"
	. "github.com/okira-e/veriflow/app/opt"
	"github.com/spf13/cobra"
)

type addCmdFlags struct {
	Flow              string `validate:"required"`
	Method            string `validate:"required,oneof=GET POST PUT PATCH DELETE OPTIONS HEAD"`
	Path              string `validate:"required,startswith=/"`
	Json              string
	Xml               string
	Files             []string // format: "fieldName:path/to/file.pdf"
	Headers           []string // format: "Header-Name:value"
	Status            int      `validate:"required,gt=99,lt=600"`
	NoSave            bool
	AssertExpressions []string
	ExportExpressions []string
}

func newAddCmd() *cobra.Command {
	var flags addCmdFlags

	cmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Add a new test step",
		Long: `
Add a new test step to a flow.

File uploads (mutually exclusive with --json and --xml):
  --file <fieldName>:<path>
    Example: --file "avatar:test-files/avatar.jpg"
    Example: --file "doc:docs/resume.pdf" --file "image:images/photo.png"

  Notes:
  - Paths are relative to veriflow.json location
  - Files must be under 100MB
  - MIME types auto-detected from file extension

Custom headers:
  --header <Header-Name>:<value>
    Example: --header "Authorization:Bearer token123"
    Example: --header "X-API-Key:secret" --header "Accept:application/json"

  Notes:
  - Custom headers can override auto-generated headers (e.g., Content-Type)
  - Use with any request type (json/xml/files)

Assertions syntax (JSON with JSONPath or XML with XPath):
  exists <path>
    Example: --assert "exists $.data.token"
    Example: --assert "exists /response/data/token"

  equals <path> <value>
    Example: --assert "equals $.user.id 42"
    Example: --assert "equals /response/user/id 42"

  contains <path> <value>
    Example: --assert "contains $.roles admin"
    Example: --assert "contains /response/roles admin"

  isNot <path> <value>
  	Example: --assert "isNot $.status PENDING"
  	Example: --assert "isNot /response/status PENDING"

  length <path> <value>
   	Example: --assert "length $.activeUsers 3"
   	Example: --assert "length /response/activeUsers 3"

Exports syntax (JSONPath for JSON, XPath for XML):
  <varname> <path>
    Example: --export "user_id $.data.user_id"
    Example: --export "user_id /response/data/user_id"
    Example: --export "token $.data.token"
`,

		Run: func(cmd *cobra.Command, args []string) {
			err := runAddCmd(cmd, args, flags)
			cli.HandleCliError(err)
		},
	}

	cmd.Flags().StringVar(&flags.Flow, "flow", "", "Flow this step belongs to")
	cmd.Flags().StringVar(&flags.Method, "method", "", "HTTP method")
	cmd.Flags().StringVar(&flags.Path, "path", "", "Request path")
	cmd.Flags().StringVar(&flags.Json, "json", "", "JSON body (optional, mutually exclusive with --xml and --file)")
	cmd.Flags().StringVar(&flags.Xml, "xml", "", "XML body (optional, mutually exclusive with --json and --file)")
	cmd.Flags().StringArrayVar(&flags.Files, "file", []string{}, "File upload (format: fieldName:path/to/file.pdf, mutually exclusive with --json and --xml)")
	cmd.Flags().StringArrayVar(&flags.Headers, "header", []string{}, "Custom HTTP header (format: Header-Name:value, repeatable)")
	cmd.Flags().IntVar(&flags.Status, "status", 0, "Asserted HTTP status code")
	cmd.Flags().StringArrayVar(&flags.AssertExpressions, "assert", []string{}, "Asserted result body")
	cmd.Flags().StringArrayVar(&flags.ExportExpressions, "export", []string{}, "Export variable from response (format: varname path)")
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

	printer := logging.NewPrinter()
	msg := fmt.Sprintf("Step \"%s\" has been added to the \"%s\" flow", stepName, flow.Name)
	printer.Println(logging.Success, msg)

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
	if flags.Json == "" && flags.Xml == "" && len(flags.Files) == 0 && flags.Method != "GET" {
		// Ask user if they want JSON, XML, or Files
		bodyTypeOptions := huh.NewOptions("json", "xml", "files", "none")
		bodyType, err := cli.PromptForOption("Request body type (json/xml/files/none)", bodyTypeOptions, false)
		if err != nil {
			return err
		}

		switch bodyType {
		case "json":
			var err error
			flags.Json, err = cli.PromptForJson("JSON to send", "", false)
			if err != nil {
				return err
			}
		case "xml":
			var err error
			flags.Xml, err = cli.PromptForString("XML to send", "", false)
			if err != nil {
				return err
			}
		case "files":
			if err := promptForFiles(flags); err != nil {
				return err
			}
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

	if len(flags.Headers) == 0 {
		if err := promptForHeaders(flags); err != nil {
			return err
		}
	}

	return nil
}

func promptForFiles(flags *addCmdFlags) error {
	files := []string{}

	for {
		// Prompt for field name
		fieldName, err := cli.PromptForString("File field name (e.g., avatar, document)", "file", true)
		if err != nil {
			return err
		}
		fieldName = strings.TrimSpace(fieldName)
		if fieldName == "" {
			break
		}

		// Prompt for file path
		filepath, err := cli.PromptForString("File path (relative to veriflow.json)", "test-files/sample.pdf", true)
		if err != nil {
			return err
		}
		filepath = strings.TrimSpace(filepath)
		if filepath == "" {
			break
		}

		// Add to files list in format "fieldName:path"
		fileSpec := fmt.Sprintf("%s:%s", fieldName, filepath)
		files = append(files, fileSpec)

		// Ask if they want to add another file
		addAnother, err := cli.PromptForBool("Add another file?")
		if err != nil {
			return err
		}
		if !addAnother {
			break
		}
	}

	flags.Files = files
	return nil
}

func promptForHeaders(flags *addCmdFlags) error {
	addHeaders, err := cli.PromptForBool("Add custom headers?")
	if err != nil {
		return err
	}

	if !addHeaders {
		return nil
	}

	headers := []string{}

	for {
		// Prompt for header type (common ones + custom)
		headerTypeOptions := huh.NewOptions("Authorization", "X-API-Key", "Accept", "Content-Type", "Custom")
		headerType, err := cli.PromptForOption("Header type", headerTypeOptions, true)
		if err != nil {
			return err
		}

		var headerName, headerValue string

		switch headerType {
		case "Authorization":
			headerName = "Authorization"
			headerValue, err = cli.PromptForString("Authorization value (e.g., Bearer token123)", "Bearer token123", true)
			if err != nil {
				return err
			}
		case "X-API-Key":
			headerName = "X-API-Key"
			headerValue, err = cli.PromptForString("API Key value", "your-api-key", true)
			if err != nil {
				return err
			}
		case "Accept":
			headerName = "Accept"
			acceptOptions := huh.NewOptions("application/json", "application/xml", "text/html", "*/*")
			headerValue, err = cli.PromptForOption("Accept value", acceptOptions, false)
			if err != nil {
				return err
			}
		case "Content-Type":
			headerName = "Content-Type"
			headerValue, err = cli.PromptForString("Content-Type value", "application/json", true)
			if err != nil {
				return err
			}
		case "Custom":
			headerName, err = cli.PromptForString("Header name", "X-Custom-Header", true)
			if err != nil {
				return err
			}
			headerName = strings.TrimSpace(headerName)
			if headerName == "" {
				continue
			}

			headerValue, err = cli.PromptForString("Header value", "", true)
			if err != nil {
				return err
			}
		default:
			continue
		}

		headerValue = strings.TrimSpace(headerValue)
		if headerValue == "" {
			break
		}

		// Add to headers list in format "Header-Name:value"
		headerSpec := fmt.Sprintf("%s:%s", headerName, headerValue)
		headers = append(headers, headerSpec)

		// Ask if they want to add another header
		addAnother, err := cli.PromptForBool("Add another header?")
		if err != nil {
			return err
		}
		if !addAnother {
			break
		}
	}

	flags.Headers = headers
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

	for {
		// Prompt for assertion type
		assertionTypeOptions := huh.NewOptions("exists", "equals", "contains", "isNot")
		assertionType, err := cli.PromptForOption("Assertion type (exists/equals/contains/isNot)", assertionTypeOptions, true)
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
		case "isNot":
			value, err := cli.PromptForString(fmt.Sprintf("Substring to check for in %s", jsonPath), "", true)
			if err != nil {
				return err
			}
			assertionExpr = fmt.Sprintf("isNot %s %s", jsonPath, value)
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
	// Make sure only one body type is sent
	bodyCount := 0
	if flags.Json != "" {
		bodyCount += 1
	}
	if flags.Xml != "" {
		bodyCount += 1
	}
	if len(flags.Files) > 0 {
		bodyCount += 1
	}
	if bodyCount > 1 {
		return nil, oops.Err(oops.ErrInvalidInput, "--json, --xml, and --file flags are mutually exclusive", nil)
	}

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
		// Build the assertions like: equals, contain, and isNot from the CLI expression.
		all, err := BuildAssertObjectFromExpressions(flags.AssertExpressions)
		if err != nil {
			return nil, oops.Err(oops.AssertionExpressionParsingFailure, "failed to parse the assertion expression", err)
		}

		assert = app.NewAssert(flags.Status, Some(all))
	} else {
		// Nothing to assert other than the status of the response.
		assert = app.NewAssert(flags.Status, None[[]*app.Assertion]())
	}

	request := app.Request{
		Method: flags.Method,
		Path:   flags.Path,
	}

	if parsedJson != nil {
		request.Json = Some[any](parsedJson)
	} else if flags.Xml != "" {
		request.Xml = Some(flags.Xml)
	} else if len(flags.Files) > 0 {
		// Parse files format "fieldName:path/to/file.pdf" into map
		filesMap := make(map[string]string)
		for _, fileSpec := range flags.Files {
			parts := strings.SplitN(fileSpec, ":", 2)
			if len(parts) != 2 {
				return nil, oops.Err(oops.ErrInvalidInput, fmt.Sprintf("invalid file format: %s (expected fieldName:path)", fileSpec), nil)
			}
			fieldName := strings.TrimSpace(parts[0])
			filePath := strings.TrimSpace(parts[1])
			if fieldName == "" || filePath == "" {
				return nil, oops.Err(oops.ErrInvalidInput, fmt.Sprintf("invalid file format: %s (field name and path cannot be empty)", fileSpec), nil)
			}
			filesMap[fieldName] = filePath
		}
		request.Files = Some(filesMap)
	}

	if len(flags.Headers) > 0 {
		// Parse headers format "Header-Name:value" into map
		headersMap := make(map[string]string)
		for _, headerSpec := range flags.Headers {
			parts := strings.SplitN(headerSpec, ":", 2)
			if len(parts) != 2 {
				return nil, oops.Err(oops.ErrInvalidInput, fmt.Sprintf("invalid header format: %s (expected Header-Name:value)", headerSpec), nil)
			}
			headerName := strings.TrimSpace(parts[0])
			headerValue := strings.TrimSpace(parts[1])
			if headerName == "" || headerValue == "" {
				return nil, oops.Err(oops.ErrInvalidInput, fmt.Sprintf("invalid header format: %s (header name and value cannot be empty)", headerSpec), nil)
			}
			headersMap[headerName] = headerValue
		}
		request.Headers = Some(headersMap)
	}

	exports, err := BuildExportsFromExpressions(flags.ExportExpressions)
	if err != nil {
		return nil, oops.Err(oops.ErrInvalidInput, "failed to parse export expressions", err)
	}

	step := app.NewStep(stepName, request, assert, exports)

	return step, nil
}

// BuildAssertObjectFromExpressions converts CLI --assert expressions into []app.Assertion.
// Valid forms:
//
//	exists   <jsonpath|xpath>
//	equals   <jsonpath|xpath> <value>
//	contains <jsonpath|xpath> <value>
//	isNot    <jsonpath|xpath> <value>
//	length   <jsonpath|xpath> <value>
//
// Supports both JSONPath ($.path) and XPath (/path)
func BuildAssertObjectFromExpressions(assertExpr []string) ([]*app.Assertion, error) {
	// Patterns (case-insensitive). Uses RE2 via Go's regexp package.
	var (
		// Match paths starting with $ (JSONPath) or / (XPath)
		reExists = regexp.MustCompile(`(?i)^\s*exists\s+([\$/][^\s]+)\s*$`)
		// string with value that can be:
		//  - "double quoted"
		//  - 'single quoted'
		//  - or unquoted (read until end, then trim whitespace)
		reWithVal = regexp.MustCompile(
			`(?i)^\s*(equals|contains|isNot|length)\s+([\$/][^\s]+)\s+(?:(?:"([^"]*)")|(?:'([^']*)')|(.+))\s*$`,
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
			if err := validatePath(path); err != nil {
				return nil, fmt.Errorf("assertion #%d: %w", i+1, err)
			}
			assertion := app.Assertion{}
			if strings.HasPrefix(path, "$") {
				assertion.JsonPath = path
			} else if strings.HasPrefix(path, "/") {
				assertion.XPath = path
			}
			assertion.Exists = Some(true)
			assertion.Contains = None[string]()
			assertion.Equals = None[string]()
			assertion.IsNot = None[string]()
			assertion.Len = None[int]()
			asserts = append(asserts, &assertion)
			continue
		}

		// contains / isNot / length
		if m := reWithVal.FindStringSubmatch(s); m != nil {
			kind := m[1]
			path := m[2]
			val := firstNonEmpty(m[3], m[4], strings.TrimSpace(m[5]))

			if err := validatePath(path); err != nil {
				return nil, fmt.Errorf("assertion #%d: %w", i+1, err)
			}
			if val == "" {
				return nil, fmt.Errorf("assertion #%d: missing VALUE for %q", i+1, kind)
			}

			assertion := app.Assertion{Exists: Some(true)}
			if strings.HasPrefix(path, "$") {
				assertion.JsonPath = path
			} else if strings.HasPrefix(path, "/") {
				assertion.XPath = path
			}

			switch kind {
			case "equals":
				assertion.Equals = Some(val)
				assertion.Contains = None[string]()
				assertion.IsNot = None[string]()

			case "contains":
				assertion.Contains = Some(val)
				assertion.Equals = None[string]()
				assertion.IsNot = None[string]()

			case "isNot":
				assertion.IsNot = Some(val)
				assertion.Equals = None[string]()
				assertion.Contains = None[string]()

			case "length":
				length, err := strconv.Atoi(val)
				if err != nil {
					return nil, fmt.Errorf("assertion #%d: invalid length value %q: %w", i+1, val, err)
				}

				assertion.IsNot = None[string]()
				assertion.Equals = None[string]()
				assertion.Contains = None[string]()
				assertion.Len = Some(length)
			default:
				// Should never happen due to regex
				return nil, fmt.Errorf("assertion #%d: unsupported type %q", i+1, kind)
			}

			asserts = append(asserts, &assertion)
			continue
		}

		return nil, fmt.Errorf(
			"invalid assertion syntax at #%d: %q\n. Expected one of:\n  - exists <path>\n  - equals <path> <value>\n  - contains <path> <value>\n  - isNot <path> <value>\n - length <path> <value>\nWhere <path> is JSONPath ($.path) or XPath (/path)",
			i+1, raw,
		)
	}

	return asserts, nil
}

// BuildExportsFromExpressions converts CLI --export expressions into app.Exports.
// Valid form: "varname jsonpath"
// Example: "user_id $.data.user_id"
func BuildExportsFromExpressions(exportExpr []string) (app.Exports, error) {
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

func validatePath(p string) error {
	if p == "" {
		return fmt.Errorf("path cannot be empty")
	}
	if p[0] != '$' && p[0] != '/' {
		return fmt.Errorf("path must start with '$' (JSONPath) or '/' (XPath): %q", p)
	}
	if strings.ContainsAny(p, " \t\r\n") {
		return fmt.Errorf("path must not contain whitespace: %q", p)
	}
	return nil
}
