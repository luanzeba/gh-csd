// Package gh provides helpers for interacting with the GitHub CLI.
package gh

import (
	"encoding/json"
	"fmt"
)

// Codespace represents a GitHub Codespace.
type Codespace struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	State       string `json:"state"`
	Repository  string `json:"repository"`
	Branch      string `json:"gitStatus.ref"`
	MachineName string `json:"machine.displayName"`
}

// codespaceJSON is used for parsing the gh cs list output
type codespaceJSON struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	State       string `json:"state"`
	Repository  string `json:"repository"`
	GitStatus   struct {
		Ref string `json:"ref"`
	} `json:"gitStatus"`
	MachineName string `json:"machineName"`
}

// ListCodespaces returns all codespaces for the authenticated user.
func ListCodespaces() ([]Codespace, error) {
	result, err := Run("cs", "list", "--json", "name,displayName,state,repository,gitStatus,machineName")
	if err != nil {
		return nil, err
	}

	var raw []codespaceJSON
	if err := json.Unmarshal(result.Stdout, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse codespaces: %w", err)
	}

	codespaces := make([]Codespace, len(raw))
	for i, cs := range raw {
		codespaces[i] = Codespace{
			Name:        cs.Name,
			DisplayName: cs.DisplayName,
			State:       cs.State,
			Repository:  cs.Repository,
			Branch:      cs.GitStatus.Ref,
			MachineName: cs.MachineName,
		}
	}

	return codespaces, nil
}

// CodespaceExists checks if a codespace with the given name exists.
func CodespaceExists(name string) (bool, error) {
	codespaces, err := ListCodespaces()
	if err != nil {
		return false, err
	}

	for _, cs := range codespaces {
		if cs.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// GetCodespace returns the codespace with the given name.
func GetCodespace(name string) (*Codespace, error) {
	codespaces, err := ListCodespaces()
	if err != nil {
		return nil, err
	}

	for _, cs := range codespaces {
		if cs.Name == name {
			return &cs, nil
		}
	}
	return nil, fmt.Errorf("codespace %q not found", name)
}
