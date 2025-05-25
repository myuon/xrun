package main

import (
	"os"
	"testing"
)

func TestProcessDataFileWithDryRun(t *testing.T) {
	tests := []struct {
		name           string
		csvContent     string
		execTemplate   string
		dryRun         bool
		expectExecution bool
	}{
		{
			name: "dry run mode should print commands only",
			csvContent: `name,age
Alice,30
Bob,25`,
			execTemplate: "echo {{.name}} is {{.age}} years old",
			dryRun:      true,
			expectExecution: false,
		},
		{
			name: "normal mode should execute commands",
			csvContent: `name,age
Alice,30
Bob,25`,
			execTemplate: "echo {{.name}} is {{.age}} years old",
			dryRun:      false,
			expectExecution: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary CSV file
			tmpFile, err := os.CreateTemp("", "test_*.csv")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.csvContent); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Track executed commands and printed commands
			var executedCommands []string
			var printedCommands []string
			
			originalExecuteCommand := func(command string) error {
				executedCommands = append(executedCommands, command)
				return nil
			}
			
			originalPrintCommand := func(command string) error {
				printedCommands = append(printedCommands, command)
				return nil
			}

			// Mock the executor functions
			if tt.dryRun {
				err = processDataFileWithExecutor(tmpFile.Name(), tt.execTemplate, originalPrintCommand)
			} else {
				err = processDataFileWithExecutor(tmpFile.Name(), tt.execTemplate, originalExecuteCommand)
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify behavior based on dry run flag
			if tt.dryRun {
				if len(executedCommands) > 0 {
					t.Errorf("Dry run mode should not execute commands, but executed: %v", executedCommands)
				}
				if len(printedCommands) == 0 {
					t.Error("Dry run mode should print commands, but none were printed")
				}
				expectedCommands := []string{
					"echo Alice is 30 years old",
					"echo Bob is 25 years old",
				}
				if len(printedCommands) != len(expectedCommands) {
					t.Errorf("Expected %d printed commands, got %d", len(expectedCommands), len(printedCommands))
				}
				for i, expected := range expectedCommands {
					if i < len(printedCommands) && printedCommands[i] != expected {
						t.Errorf("Printed command %d: expected %q, got %q", i, expected, printedCommands[i])
					}
				}
			} else {
				if len(printedCommands) > 0 {
					t.Errorf("Normal mode should not print commands, but printed: %v", printedCommands)
				}
				if len(executedCommands) == 0 {
					t.Error("Normal mode should execute commands, but none were executed")
				}
			}
		})
	}
}