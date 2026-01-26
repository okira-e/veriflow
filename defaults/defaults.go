package defaults

import _ "embed"

//go:embed default-config.json
var DefaultConfig []byte
