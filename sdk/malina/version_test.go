package malina_test

import (
	"testing"

	"github.com/ardanlabs/malina/sdk/malina"
)

func TestVersion(t *testing.T) {
	version := malina.Version
	if version == "" {
		t.Fatal("version returned an empty string, which is invalid")
	}
	t.Logf("version returned: %s", version)
}
