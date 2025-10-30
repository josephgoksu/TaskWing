package cmd

import (
	"testing"

	"github.com/spf13/pflag"
)

func resetClearCommandState(t *testing.T) {
	t.Helper()

	clearCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if err := f.Value.Set(f.DefValue); err != nil {
			t.Fatalf("reset flag %s: %v", f.Name, err)
		}
		f.Changed = false
	})

	clearForce = false
	clearStatus = ""
	clearPriority = ""
	clearCompleted = false
	clearAll = false
	clearBackup = true
	clearNoBackup = false
}

func TestClearBackupDefaultsEnabled(t *testing.T) {
	resetClearCommandState(t)

	if err := clearCmd.PreRunE(clearCmd, nil); err != nil {
		t.Fatalf("prerun: %v", err)
	}

	if !clearBackup {
		t.Fatalf("expected clearBackup to default to true")
	}
}

func TestClearNoBackupFlagDisablesBackup(t *testing.T) {
	resetClearCommandState(t)

	if err := clearCmd.Flags().Set("no-backup", "true"); err != nil {
		t.Fatalf("set no-backup: %v", err)
	}

	if err := clearCmd.PreRunE(clearCmd, nil); err != nil {
		t.Fatalf("prerun: %v", err)
	}

	if clearBackup {
		t.Fatalf("expected clearBackup to be false when no-backup is set")
	}
}

func TestClearBackupFlagCanDisableBackup(t *testing.T) {
	resetClearCommandState(t)

	if err := clearCmd.Flags().Set("backup", "false"); err != nil {
		t.Fatalf("set backup: %v", err)
	}

	if err := clearCmd.PreRunE(clearCmd, nil); err != nil {
		t.Fatalf("prerun: %v", err)
	}

	if clearBackup {
		t.Fatalf("expected clearBackup to respect --backup=false")
	}
}
