package step

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/cli"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/utils"
	"github.com/spf13/cobra"
)

type createCmdFlags struct {
	flow   string
	method string
	path   string
	json   string
	status int
}

func newCreateCmd() *cobra.Command {
	var flags createCmdFlags

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new test step",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCreateCmd(cmd, args, flags)
		},
	}

	cmd.Flags().StringVar(&flags.flow, "flow", "", "Flow this step belongs to")
	cmd.Flags().StringVar(&flags.method, "method", "", "HTTP method")
	cmd.Flags().StringVar(&flags.path, "path", "", "Request path")
	cmd.Flags().StringVar(&flags.json, "json", "", "JSON body (optional)")
	cmd.Flags().IntVar(&flags.status, "status", 0, "Expected HTTP status code")

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
			return err
		}
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("Failed to load config: %s.", err)
	}

	// Make sure there's at least one flow to add a step to.
	if len(cfg.Flows) == 0 {
		utils.ErrorOut("EMPTY_FLOWS", "You need at least one flow to add a step to. Create a flow with `veriflow flow create [name].`")
		os.Exit(2)
	}

	if err := promptForRequiredFlags(&flags); err != nil {
		return fmt.Errorf("Failed to validate required flags.")
	}

	// Only prompt for optional parameters if --no-interactive is not provided.
	// Otherwise, assume the bot provided everything it needs and nothing more.
	if !cliopts.NonInteractive {
		if err := promptForOptionalFlags(&flags); err != nil {
			return fmt.Errorf("Failed to validate optional flags.")
		}
	}

	// Validate the flow provided exists.
	flow, ok := cfg.GetFlow(flags.flow)
	if !ok {
		utils.ErrorOut("FLOW_DOESNT_EXIST", "Flow provided doesn't exist.\n")
		os.Exit(2)
	}

	step, err := buildStepFromFlags(stepName, &flags)
	if err != nil {
		return fmt.Errorf("Failed to build step from flags: %s.\n", err)
	}

	err = flow.AddStep(step)
	if err != nil {
		return err
	}

	err = cfg.UpdateFlow(flow)
	if err != nil {
		fmt.Errorf("Failed to update the config: %s.\n", err)
	}

	return nil
}

func promptForRequiredFlags(flags *createCmdFlags) error {
	if flags.flow == "" {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("Failed to load config: %s.", err)
		}

		flowNames := make([]string, len(cfg.Flows))
		for i, flowPtr := range cfg.Flows {
			flowNames[i] = flowPtr.Name
		}

		options := huh.NewOptions(flowNames...)
		flags.flow, err = cli.PromptForOption("Which flow does this belong to?", options, true)
		if err != nil {
			return fmt.Errorf("Failed to prompt for flow name: %s", err)
		}
	}

	if flags.method == "" {
		var err error
		flags.method, err = cli.PromptForString("Method", "POST", true)
		if err != nil {
			return fmt.Errorf("Failed to prompt for method: %s", err)
		}
	}

	if flags.path == "" {
		var err error
		flags.path, err = cli.PromptForString("Path", "/users/register", true)
		if err != nil {
			return fmt.Errorf("Failed to prompt for path: %s", err)
		}
	}

	if flags.status == 0 {
		var err error
		flags.status, err = cli.PromptForInt("Status to expect", "201", true)
		if err != nil {
			return fmt.Errorf("Failed to prompt for path: %s", err)
		}
	}

	return nil
}

func promptForOptionalFlags(flags *createCmdFlags) error {
	if flags.json == "" {
		var err error
		flags.json, err = cli.PromptForJson("JSON to send", "", false)
		if err != nil {
			return fmt.Errorf("Failed to prompt for json: %s", err)
		}
	}

	return nil
}

func buildStepFromFlags(stepName string, flags *createCmdFlags) (*app.Step, error) {

	var parsedJson map[string]any = nil
	if flags.json != "" {
		if err := json.Unmarshal([]byte(flags.json), &parsedJson); err != nil {
			return nil, fmt.Errorf("Failed to parse the json passed for the request: %s\n", err)
		}
	}
	request := app.NewRequest(flags.method, flags.path, parsedJson)

	expect := app.NewExpect(flags.status)

	exports := app.Exports{}

	step := app.NewStep(stepName, request, expect, exports)

	return step, nil
}
