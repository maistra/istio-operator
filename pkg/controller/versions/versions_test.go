package versions

import (
	"testing"
)

func TestStringsDefinedForAllVersions(t *testing.T) {
	// This test verifies that string mappings have been provided for all defined versions
	for vint := 0; vint <= int(lastKnownVersion); vint++ {
		if _, ok := versionToString[Ver(vint)]; !ok {
			t.Errorf("no version string defined for version: %d", vint)
		}
	}
}

func TestStrategyDefinedForAllVersions(t *testing.T) {
	// This test verifies that string mappings have been provided for all defined versions
	for vint := 0; vint <= int(lastKnownVersion); vint++ {
		if strategy, ok := versionToStrategy[Ver(vint)]; !ok {
			t.Errorf("no strategy defined for version: %d", vint)
		} else if strategy == nil {
			t.Errorf("strategy for version is nil: %d", vint)
		}
	}
}

func TestBadVersionString(t *testing.T) {
	// This test verifies that the parser returns an error for invalid versions
	if _, err := ParseVersion("InvalidVersion"); err == nil {
		t.Errorf("ParseVersion() should have returned an error for version InvalidVersion")
	}
}
