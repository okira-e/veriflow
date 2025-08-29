package internal

type Step struct {
	Name    string  `json:"name"`
	Request Request `json:"request"`
	Expect  Expect  `json:"expect"`
	Exports Exports `json:"exports"`
}
