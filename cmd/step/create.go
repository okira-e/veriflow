package step

import (
	"fmt"
	"log"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/okira-e/veriflow/internal"
	"github.com/okira-e/veriflow/internal/cli"
	"github.com/okira-e/veriflow/internal/config"
	"github.com/okira-e/veriflow/internal/utils"
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
		Args:  cobra.ExactArgs(1),
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
	stepName := args[0]

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("Failed to load config: %s.", err)
	}

	// Make sure there's at least one flow to add a step to.
	if len(cfg.Flows) == 0 {
		utils.ErrorOut("EMPTY_FLOWS", "You need at least one flow to add a step to. Create a flow with `veriflow flow create [name].`")
		os.Exit(2)
	}

	var step internal.Step

	if err := validateRequiredFlags(&flags, &step); err != nil {
		return fmt.Errorf("Failed to validate required flags.")
	}

	// buildStepFromFlags(flags, &step)

	// if err := promptForMissingData(data, cfg, flags); err != nil {
	// 	return nil, err
	// }

	// Validate the flow provided exists.
	if _, ok := cfg.GetFlow(flags.flow); !ok {
		return fmt.Errorf("Flow provided doesn't exist.\n")
	}

	fmt.Println("STEP NAME: ", stepName)
	fmt.Println("METHOD: ", flags.method)

	return nil
}

func validateRequiredFlags(flags *createCmdFlags, step *internal.Step) error {
	if flags.flow == "" {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("Failed to load config: %s.", err)
		}

		flowNames := make([]string, len(cfg.Flows))
		for _, flowPtr := range cfg.Flows {
			flowNames = append(flowNames, flowPtr.Name)
		}

		options := huh.NewOptions(flowNames...)
		flow, err := cli.PromptForOption("Flow name", options, true)
		if err != nil {
			return fmt.Errorf("Failed to prompt for flow name: %s", err)
		}

		log.Fatalf("FLOW BABY: %s\n", flow)
	}

	if flags.method == "" {
		cli.PromptForString("Method", "POST", true)
	}

	if flags.path == "" {

	}

	if flags.json == "" {

	}

	if flags.status == 0 {

	}

	return nil
}
