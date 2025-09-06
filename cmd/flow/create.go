package flow

import (
	"fmt"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/cli"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/utils"
	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new flow",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runCreateCmd(cmd, args)
			utils.HandleCliError(err)
			return err
		},
	}

	return cmd
}

func runCreateCmd(cmd *cobra.Command, args []string) error {
	var flowName string

	if len(args) > 0 {
		flowName = args[0]
	}

	if flowName == "" {
		var err error
		flowName, err = cli.PromptForString("Flow name", "user-onboarding", true)
		if err != nil {
			return err
		}
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	flow := app.NewFlow(flowName)

	if err := cfg.AddFlow(flow); err != nil {
		return err
	}

	msg := fmt.Sprintf("Flow \"%s\" has been added to veriflow.json", flowName)
	utils.PrintInColor("green", msg)

	return nil
}
