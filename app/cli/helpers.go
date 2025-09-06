package cli

func isAborted(err error) bool {
	if err == nil {
		return false
	}

	if err.Error() == "user aborted" {
		return true
	}

	return false
}
