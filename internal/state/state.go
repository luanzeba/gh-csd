// Package state manages the current codespace selection.
// State is stored in ~/.csd/current which contains the codespace name.
package state

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	stateDirName  = ".csd"
	stateFileName = "current"
)

var (
	ErrNoCodespace = errors.New("no codespace selected")
)

// stateDir returns the path to the state directory (~/.csd)
func stateDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, stateDirName), nil
}

// stateFile returns the path to the state file (~/.csd/current)
func stateFile() (string, error) {
	dir, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, stateFileName), nil
}

// Get returns the currently selected codespace name.
// Returns ErrNoCodespace if no codespace is selected.
func Get() (string, error) {
	path, err := stateFile()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNoCodespace
		}
		return "", err
	}

	name := strings.TrimSpace(string(data))
	if name == "" {
		return "", ErrNoCodespace
	}

	return name, nil
}

// Set saves the given codespace name as the current selection.
func Set(name string) error {
	dir, err := stateDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path, err := stateFile()
	if err != nil {
		return err
	}

	return os.WriteFile(path, []byte(name+"\n"), 0644)
}

// Clear removes the current codespace selection.
func Clear() error {
	path, err := stateFile()
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
