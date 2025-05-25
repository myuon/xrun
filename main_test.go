package main

import (
	"os"
	"strings"
	"testing"
)

func TestProcessCSVWithExecutor(t *testing.T) {
	// Create a temporary CSV file for testing
	csvContent := `user_id,name,email
1,John Doe,john@example.com
2,Jane Smith,jane@example.com`

	tmpFile, err := os.CreateTemp("", "test_*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(csvContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Track executed commands
	var executedCommands []string
	mockExecutor := func(command string) error {
		executedCommands = append(executedCommands, command)
		return nil
	}

	// Test template substitution
	execTemplate := "echo {{.name}} {{.email}}"
	
	err = processCSVWithExecutor(tmpFile.Name(), execTemplate, mockExecutor)
	if err != nil {
		t.Fatalf("processCSVWithExecutor failed: %v", err)
	}

	// Verify expected commands were executed
	expectedCommands := []string{
		"echo John Doe john@example.com",
		"echo Jane Smith jane@example.com",
	}

	if len(executedCommands) != len(expectedCommands) {
		t.Fatalf("Expected %d commands, got %d", len(expectedCommands), len(executedCommands))
	}

	for i, expected := range expectedCommands {
		if executedCommands[i] != expected {
			t.Errorf("Command %d: expected %q, got %q", i, expected, executedCommands[i])
		}
	}
}

func TestProcessCSVWithExecutor_TemplateError(t *testing.T) {
	// Create a temporary CSV file for testing
	csvContent := `user_id,name,email
1,John Doe,john@example.com`

	tmpFile, err := os.CreateTemp("", "test_*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(csvContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Track executed commands
	var executedCommands []string
	mockExecutor := func(command string) error {
		executedCommands = append(executedCommands, command)
		return nil
	}

	// Test with invalid template
	execTemplate := "echo {{.invalid_field}}"
	
	err = processCSVWithExecutor(tmpFile.Name(), execTemplate, mockExecutor)
	if err != nil {
		t.Fatalf("processCSVWithExecutor failed: %v", err)
	}

	// Should still process the row but with empty substitution
	expectedCommands := []string{
		"echo ",
	}

	if len(executedCommands) != len(expectedCommands) {
		t.Fatalf("Expected %d commands, got %d", len(expectedCommands), len(executedCommands))
	}

	for i, expected := range expectedCommands {
		if executedCommands[i] != expected {
			t.Errorf("Command %d: expected %q, got %q", i, expected, executedCommands[i])
		}
	}
}

func TestProcessCSVWithExecutor_FileNotFound(t *testing.T) {
	mockExecutor := func(command string) error {
		return nil
	}

	err := processCSVWithExecutor("nonexistent.csv", "echo {{.name}}", mockExecutor)
	if err == nil {
		t.Fatal("Expected error for nonexistent file, got nil")
	}

	if !strings.Contains(err.Error(), "failed to open data file") {
		t.Errorf("Expected 'failed to open data file' error, got: %v", err)
	}
}

func TestProcessCSVWithExecutor_InvalidTemplate(t *testing.T) {
	// Create a temporary CSV file for testing
	csvContent := `user_id,name,email
1,John Doe,john@example.com`

	tmpFile, err := os.CreateTemp("", "test_*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(csvContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	mockExecutor := func(command string) error {
		return nil
	}

	// Test with invalid template syntax
	execTemplate := "echo {{.name"
	
	err = processCSVWithExecutor(tmpFile.Name(), execTemplate, mockExecutor)
	if err == nil {
		t.Fatal("Expected error for invalid template, got nil")
	}

	if !strings.Contains(err.Error(), "failed to parse template") {
		t.Errorf("Expected 'failed to parse template' error, got: %v", err)
	}
}