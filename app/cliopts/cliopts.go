package cliopts

// These are set by Cobra flag binding on the root command.
var (
	ConfigFile     string // --config
	JSONOutput     bool   // --json-output
	NoColor        bool   // --no-color or NO_COLOR
	NonInteractive bool   // --no-interactive
	Verbose        bool   // --verbose
	Silent         bool   // --silent
)
