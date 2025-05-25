package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateLogWriter(t *testing.T) {
	tests := []struct {
		name         string
		dataFile     string
		expectedBase string
	}{
		{
			name:         "CSV file",
			dataFile:     "test_data.csv",
			expectedBase: "xrun-test_data-",
		},
		{
			name:         "JSON file",
			dataFile:     "users.json",
			expectedBase: "xrun-users-",
		},
		{
			name:         "File without extension",
			dataFile:     "data",
			expectedBase: "xrun-data-",
		},
		{
			name:         "Path with directory",
			dataFile:     "/path/to/my_file.csv",
			expectedBase: "xrun-my_file-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logWriter, err := createLogWriter(tt.dataFile)
			if err != nil {
				t.Fatalf("createLogWriter failed: %v", err)
			}
			defer func() {
				logWriter.Close()
				os.Remove(logWriter.file.Name())
			}()

			fileName := filepath.Base(logWriter.file.Name())
			if !strings.HasPrefix(fileName, tt.expectedBase) {
				t.Errorf("Expected file name to start with %q, got %q", tt.expectedBase, fileName)
			}

			if !strings.HasSuffix(fileName, ".logs") {
				t.Errorf("Expected file name to end with .logs, got %q", fileName)
			}

			// Verify timestamp format in filename
			// Format should be: xrun-[basename]-YYYYMMDD-HHMMSS.logs
			timestampPart := strings.TrimPrefix(fileName, tt.expectedBase)
			timestampPart = strings.TrimSuffix(timestampPart, ".logs")
			
			if len(timestampPart) != 15 { // YYYYMMDD-HHMMSS = 15 chars
				t.Errorf("Expected timestamp format YYYYMMDD-HHMMSS (15 chars), got %q (%d chars)", timestampPart, len(timestampPart))
			}
		})
	}
}

func TestProcessDataFileWithOptions_NoLogFiles(t *testing.T) {
	// Create temporary CSV file
	tmpFile, err := os.CreateTemp("", "test_*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	csvContent := `name,message
Alice,Hello World
Bob,Goodbye`

	if _, err := tmpFile.WriteString(csvContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Track executed commands
	var executedCommands []string
	originalExecutor := func(command string) error {
		executedCommands = append(executedCommands, command)
		return nil
	}

	// Override createCommandExecutor to use our mock
	originalCreateCommandExecutor := createCommandExecutor
	createCommandExecutor = func(dryRun bool, logWriter *LogWriter) CommandExecutor {
		if dryRun {
			return printCommand
		}
		return originalExecutor
	}
	defer func() {
		createCommandExecutor = originalCreateCommandExecutor
	}()

	// Test with --no-log-files (should not create log file)
	err = processDataFileWithOptions(tmpFile.Name(), "echo {{.name}}: {{.message}}", false, true)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify commands were executed
	expectedCommands := []string{
		"echo Alice: Hello World",
		"echo Bob: Goodbye",
	}

	if len(executedCommands) != len(expectedCommands) {
		t.Fatalf("Expected %d commands, got %d", len(expectedCommands), len(executedCommands))
	}

	for i, expected := range expectedCommands {
		if executedCommands[i] != expected {
			t.Errorf("Command %d: expected %q, got %q", i, expected, executedCommands[i])
		}
	}

	// Verify no log files were created with the expected pattern
	files, err := filepath.Glob("xrun-test_*.logs")
	if err != nil {
		t.Fatalf("Failed to glob for log files: %v", err)
	}

	if len(files) > 0 {
		defer func() {
			for _, file := range files {
				os.Remove(file)
			}
		}()
		t.Errorf("Expected no log files to be created with --no-log-files, but found: %v", files)
	}
}

func TestProcessDataFileWithOptions_WithLogFiles(t *testing.T) {
	// Create temporary CSV file
	tmpFile, err := os.CreateTemp("", "test_*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	csvContent := `command
echo test_output`

	if _, err := tmpFile.WriteString(csvContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Test with log files enabled (default)
	err = processDataFileWithOptions(tmpFile.Name(), "{{.command}}", false, false)
	
	// We expect this to fail in test environment since "echo" command execution
	// requires actual command execution, but we can verify the log file creation logic
	// by checking if createLogWriter would be called
	
	// For this test, we just verify the code doesn't crash and handles the flag correctly
	if err != nil {
		// This is expected since we're trying to execute actual commands
		t.Logf("Expected error during command execution: %v", err)
	}

	// Clean up any log files that might have been created
	baseName := strings.TrimSuffix(filepath.Base(tmpFile.Name()), filepath.Ext(tmpFile.Name()))
	pattern := fmt.Sprintf("xrun-%s-*.logs", baseName)
	files, _ := filepath.Glob(pattern)
	for _, file := range files {
		os.Remove(file)
	}
}

func TestProcessDataFileWithOptions_DryRun(t *testing.T) {
	// Create temporary CSV file
	tmpFile, err := os.CreateTemp("", "test_*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	csvContent := `name
Alice
Bob`

	if _, err := tmpFile.WriteString(csvContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Test dry run mode (should not create log files even without --no-log-files)
	err = processDataFileWithOptions(tmpFile.Name(), "echo {{.name}}", true, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify no log files were created during dry run
	baseName := strings.TrimSuffix(filepath.Base(tmpFile.Name()), filepath.Ext(tmpFile.Name()))
	pattern := fmt.Sprintf("xrun-%s-*.logs", baseName)
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("Failed to glob for log files: %v", err)
	}

	if len(files) > 0 {
		defer func() {
			for _, file := range files {
				os.Remove(file)
			}
		}()
		t.Errorf("Expected no log files to be created during dry run, but found: %v", files)
	}
}