package maistra

import (
	"testing"
)

func TestStringsDefinedForAllVersions(t *testing.T) {
	// This test verifies that string mappings have been provided for all defined versions
	for vint := 0; vint <= int(lastKnownVersion); vint++ {
		if _, ok := versionToString[version(vint)]; !ok {
			t.Errorf("no version string defined for version: %d", vint)
		}
	}
}

func TestBadVersionString(t *testing.T) {
	// This test verifies that the parser returns an error for invalid versions
	if _, err := ParseVersion("InvalidVersion"); err == nil {
		t.Errorf("ParseVersion() should have returned an error for version InvalidVersion")
	}
}
