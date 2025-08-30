package cliopts

// These are set by Cobra flag binding on the root command.
var (
	JSON           bool // --json
	NoColor        bool // --no-color or NO_COLOR
	NonInteractive bool // --no-interactive
)
