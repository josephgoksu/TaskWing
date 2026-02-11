package cmd

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/josephgoksu/TaskWing/internal/bootstrap"
)

func TestSlashContentRegistry_CoversCanonicalCatalog(t *testing.T) {
	for _, slash := range bootstrap.SlashCommands {
		if _, ok := slashContents[slash.SlashCmd]; !ok {
			t.Fatalf("missing slash content mapping for %q", slash.SlashCmd)
		}
	}
}

func TestSlashRuntimeCommands_MatchCanonicalCatalog(t *testing.T) {
	expected := bootstrap.SlashCommandNames()
	sort.Strings(expected)

	actual := availableSlashCommands(slashCmd)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("slash command registry drift\nexpected: %v\nactual:   %v", expected, actual)
	}
}

func TestSlashUnknownCommand_UsesRuntimeAvailability(t *testing.T) {
	err := slashCmd.RunE(slashCmd, []string{"does-not-exist"})
	if err == nil {
		t.Fatal("expected unknown slash command error")
	}

	errMsg := err.Error()
	expectedList := strings.Join(availableSlashCommands(slashCmd), ", ")
	if !strings.Contains(errMsg, expectedList) {
		t.Fatalf("unknown command error should list runtime available commands %q, got %q", expectedList, errMsg)
	}
}
