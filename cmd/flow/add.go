package flow

import (
	"fmt"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/cli"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/logging"
	"github.com/spf13/cobra"
)

type addCmdFlags struct {
	NoSave bool
}

func newAddCmd() *cobra.Command {
	var flags addCmdFlags

	cmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Add a new flow",
		Run: func(cmd *cobra.Command, args []string) {
			err := runAddCmd(cmd, args, flags)
			cli.HandleCliError(err)
		},
	}

	cmd.Flags().BoolVar(&flags.NoSave, "no-save", false, "Modify the config but don't save on disk")

	return cmd
}

func runAddCmd(cmd *cobra.Command, args []string, flags addCmdFlags) error {
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

	cfg, err := config.LoadConfig(cliopts.ConfigFile)
	if err != nil {
		return err
	}

	flow := app.NewFlow(flowName)

	if err := cfg.AddFlow(flow); err != nil {
		return err
	}

	if !flags.NoSave {
		if err = cfg.Save(); err != nil {
			return err
		}
	}

	printer := logging.NewPrinter()
	msg := fmt.Sprintf("Flow \"%s\" has been added to veriflow.json", flowName)
	printer.Println(logging.Success, msg)

	return nil
}
