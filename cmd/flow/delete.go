package flow

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/okira-e/veriflow/app/cli"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/logging"
	"github.com/okira-e/veriflow/app/oops"
	"github.com/spf13/cobra"
)

type deleteCmdFlags struct {
	yes    bool
	NoSave bool
}

func newDeleteCmd() *cobra.Command {
	var flags deleteCmdFlags

	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete an existing flow",
		Run: func(cmd *cobra.Command, args []string) {
			err := runDeleteCmd(cmd, args, flags)
			cli.HandleCliError(err)
		},
	}

	cmd.Flags().BoolVarP(&flags.yes, "yes", "y", false, "confirm/accept an action")
	cmd.Flags().BoolVar(&flags.NoSave, "no-save", false, "Modify the config but don't save on disk")

	return cmd
}

func runDeleteCmd(cmd *cobra.Command, args []string, flags deleteCmdFlags) error {
	var flowName string

	if len(args) > 0 {
		flowName = args[0]
	}

	cfg, err := config.LoadConfig(cliopts.ConfigFile)
	if err != nil {
		return err
	}

	// Prompt for name if not provided.
	if flowName == "" {
		flowNames := make([]string, len(cfg.Flows))
		for i, flowPtr := range cfg.Flows {
			flowNames[i] = flowPtr.Name
		}

		options := huh.NewOptions(flowNames...)
		flowName, err = cli.PromptForOption("Flow name", options, true)
		if err != nil {
			return err
		}
	}

	flow, ok := cfg.GetFlow(flowName)
	if !ok {
		return oops.Err(oops.FlowNotFound, fmt.Sprintf("Flow with name \"%s\" doesn't exist", flowName), nil)
	}

	confirmed := flags.yes
	if !confirmed {
		msg := fmt.Sprintf(
			"Are you sure you want to delete the \"%s\" flow with %d steps?",
			flowName,
			len(flow.Steps),
		)
		confirmed, err = cli.PromptForBool(msg)
		if err != nil {
			return err
		}
	}

	if !confirmed {
		return nil
	}

	if err := cfg.RemoveFlow(flowName); err != nil {
		return err
	}

	if !flags.NoSave {
		if err = cfg.Save(); err != nil {
			return err
		}
	}

	printer := logging.NewPrinter()
	if !cliopts.Silent {
		msg := fmt.Sprintf("Flow \"%s\" has been deleted from veriflow.json", flowName)
		printer.Println(logging.Success, msg)
	}

	return nil
}
