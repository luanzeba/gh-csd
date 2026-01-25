package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetSetClear(t *testing.T) {
	// Use a temp directory for testing
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Test Get with no selection
	_, err := Get()
	if err != ErrNoCodespace {
		t.Errorf("Get() with no selection: got err=%v, want ErrNoCodespace", err)
	}

	// Test Set
	testName := "test-codespace-123"
	if err := Set(testName); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// Verify file was created
	stateFile := filepath.Join(tmpDir, ".csd", "current")
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("State file was not created")
	}

	// Test Get after Set
	got, err := Get()
	if err != nil {
		t.Fatalf("Get() after Set failed: %v", err)
	}
	if got != testName {
		t.Errorf("Get() = %q, want %q", got, testName)
	}

	// Test Clear
	if err := Clear(); err != nil {
		t.Fatalf("Clear() failed: %v", err)
	}

	// Test Get after Clear
	_, err = Get()
	if err != ErrNoCodespace {
		t.Errorf("Get() after Clear: got err=%v, want ErrNoCodespace", err)
	}
}
