package cli

import (
	"errors"
	"os"
	"strings"
)

// defaultMachineName suggests the OS hostname as a starting point for the
// "what do you want to call this machine?" prompt — still asked and
// editable, never silently used.
func defaultMachineName() string {
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return ""
}

func requireNonEmpty(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("a name is required")
	}
	return nil
}
