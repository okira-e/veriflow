package export

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/cli"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/logging"
	"github.com/okira-e/veriflow/app/oops"
	"github.com/spf13/cobra"
)

func SetupExportCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(newRootCmd())
}

type exportFlags struct {
	BaseUrlOverride string
	To              string
	Out             string
}

func newRootCmd() *cobra.Command {
	flags := &exportFlags{}

	cmd := &cobra.Command{
		Use:   "export [flow[/step] ...] --to <format>",
		Short: "Export one or more flows or specific steps into an external request format such as curl.",
		Long: `Export Veriflow flows or individual steps into external request formats for inspection or manual execution.
Flows are exported as ordered sequences; targeting flow/step exports only that step.

Output format:
- Single target: outputs raw curl command
- Multiple targets: outputs JSON array with stepName and curl fields

Note: Bindings ({{RUN_ID}}, {{bind:var}}, etc.) are NOT resolved and will be emitted as-is in the exported output.
The output format is selected via --to (currently only 'curl' is supported) and written to stdout or a file via the --out flag.
Exports are lossy and read-only - they capture the request structure but not runtime state.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := exportCommand(flags, args)
			cli.HandleCliError(err)

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&flags.BaseUrlOverride, "base-url", "", "Override the base URL in the config")
	f.StringVar(&flags.To, "to", "curl", "Export format")
	f.StringVar(&flags.Out, "out", "", "Path to write exported format to")

	return cmd
}

// exportCommand is the entry point for the export command.
// Returns (hadFailures, error) where hadFailures indicates assertion failures occurred.
func exportCommand(flags *exportFlags, args []string) error {
	cfg, err := config.LoadConfig(cliopts.ConfigFile)
	if err != nil {
		return err
	}

	targets, err := cli.ParseTargets(cfg, args)
	if err != nil {
		return err
	}
	stepTargets := cli.FlattenTargets(targets, map[string]bool{})

	baseUrl := cfg.BaseUrl
	if flags.BaseUrlOverride != "" {
		baseUrl = flags.BaseUrlOverride
	}

	exported := make([]StepExportOutput, len(stepTargets))
	switch flags.To {
	case "curl":
		{
			for i, target := range stepTargets {
				curl, err := convertStepToCurl(target.Step, baseUrl)
				if err != nil {
					return err
				}

				exported[i] = StepExportOutput{
					StepName: target.Step.Name,
					Curl:     curl,
				}
			}
		}
	default:
		{
			msg := fmt.Sprintf("Format \"%s\" is not supported", flags.To)
			return oops.Err(oops.InvalidExportFormat, msg, nil)
		}
	}

	// Output format: raw curl string for single target, JSON for multiple targets
	var outputData []byte
	if len(exported) == 1 {
		// Single target: output raw curl command
		outputData = []byte(exported[0].Curl)
	} else {
		// Multiple targets: output JSON array
		outputData, err = json.MarshalIndent(exported, "", "  ")
		if err != nil {
			return oops.Err(oops.Internal, "failed to marshal export output: %w", err)
		}
	}

	if flags.Out != "" {
		err = os.WriteFile(flags.Out, outputData, 0644)
		if err != nil {
			return oops.Err(oops.Internal, "failed to write export output to file: %w", err)
		}
	} else {
		printer := logging.NewPrinter()
		printer.Println(logging.Info, string(outputData))
	}

	return nil
}

// convertStepToCurl is an @AI generated function to generate a curl command from a config step
func convertStepToCurl(step *app.Step, baseURL string) (string, error) {
	if step == nil {
		return "", fmt.Errorf("nil step")
	}

	method := strings.ToUpper(step.Request.Method)
	if method == "" {
		method = "GET"
	}

	fullURL := strings.TrimRight(baseURL, "/") + step.Request.Path

	var b strings.Builder
	b.WriteString("curl")
	b.WriteString(" -X ")
	b.WriteString(shellEscape(method))
	b.WriteString(" ")
	b.WriteString(shellEscape(fullURL))

	// Headers
	if !step.Request.DisableHeaders {
		if step.Request.Json.IsSome() {
			b.WriteString(` -H "Content-Type: application/json"`)
		}
		if step.Request.Xml.IsSome() {
			b.WriteString(` -H "Content-Type: application/xml"`)
		}
	}

	// Custom headers (added after auto-headers so they can override)
	if step.Request.Headers.IsSome() {
		headers := step.Request.Headers.Unwrap()
		for headerName, headerValue := range headers {
			b.WriteString(" -H ")
			// Format: -H "Header-Name: value" (with space after colon)
			b.WriteString(shellEscape(headerName + ": " + headerValue))
		}
	}

	// Body
	if step.Request.Json.IsSome() {
		payload := step.Request.Json.Unwrap()
		raw, err := json.Marshal(payload)
		if err != nil {
			return "", fmt.Errorf("json marshal failed: %w", err)
		}
		b.WriteString(" --data ")
		b.WriteString(shellEscape(string(raw)))
	}

	if step.Request.Xml.IsSome() {
		b.WriteString(" --data ")
		b.WriteString(shellEscape(step.Request.Xml.Unwrap()))
	}

	// Files (multipart/form-data)
	if step.Request.Files.IsSome() {
		files := step.Request.Files.Unwrap()
		for fieldName, filePath := range files {
			b.WriteString(" -F ")
			// Format: fieldName=@path/to/file
			b.WriteString(shellEscape(fieldName + "=@" + filePath))
		}
	}

	// Timeout (curl is seconds, rounded up)
	if step.Options.Timeout.IsSome() {
		d, err := time.ParseDuration(step.Options.Timeout.Unwrap())
		if err != nil {
			return "", fmt.Errorf("invalid timeout: %w", err)
		}
		secs := int(math.Ceil(d.Seconds()))
		if secs > 0 {
			b.WriteString(" --max-time ")
			b.WriteString(strconv.Itoa(secs))
		}
	}

	return b.String(), nil
}

func shellEscape(s string) string {
	// single-quote safe escape for POSIX shells
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

type StepExportOutput struct {
	StepName string `json:"stepName"`
	Curl     string `json:"curl"`
}
