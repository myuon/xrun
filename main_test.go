package main

import (
	"os"
	"strings"
	"testing"
)

func TestProcessCSVWithExecutor(t *testing.T) {
	tests := []struct {
		name             string
		csvContent       string
		execTemplate     string
		expectedCommands []string
		expectError      bool
		errorContains    string
	}{
		{
			name: "basic template substitution",
			csvContent: `user_id,name,email
1,John Doe,john@example.com
2,Jane Smith,jane@example.com`,
			execTemplate: "echo {{.name}} {{.email}}",
			expectedCommands: []string{
				"echo John Doe john@example.com",
				"echo Jane Smith jane@example.com",
			},
			expectError: false,
		},
		{
			name: "template with missing field",
			csvContent: `user_id,name,email
1,John Doe,john@example.com`,
			execTemplate: "echo {{.invalid_field}}",
			expectedCommands: []string{
				"echo ",
			},
			expectError: false,
		},
		{
			name: "single field template",
			csvContent: `name
Alice
Bob`,
			execTemplate: "echo Hello {{.name}}",
			expectedCommands: []string{
				"echo Hello Alice",
				"echo Hello Bob",
			},
			expectError: false,
		},
		{
			name: "multiple field template",
			csvContent: `id,name,age,city
1,Alice,25,Tokyo
2,Bob,30,Osaka`,
			execTemplate: "echo {{.name}} is {{.age}} years old and lives in {{.city}}",
			expectedCommands: []string{
				"echo Alice is 25 years old and lives in Tokyo",
				"echo Bob is 30 years old and lives in Osaka",
			},
			expectError: false,
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

			// Track executed commands
			var executedCommands []string
			mockExecutor := func(command string) error {
				executedCommands = append(executedCommands, command)
				return nil
			}

			// Execute test
			err = processCSVWithExecutor(tmpFile.Name(), tt.execTemplate, mockExecutor)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if tt.expectError && err != nil {
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
				}
				return
			}

			// Verify executed commands
			if len(executedCommands) != len(tt.expectedCommands) {
				t.Fatalf("Expected %d commands, got %d", len(tt.expectedCommands), len(executedCommands))
			}

			for i, expected := range tt.expectedCommands {
				if executedCommands[i] != expected {
					t.Errorf("Command %d: expected %q, got %q", i, expected, executedCommands[i])
				}
			}
		})
	}
}

func TestProcessCSVWithExecutor_ErrorCases(t *testing.T) {
	tests := []struct {
		name          string
		csvFile       string
		execTemplate  string
		createFile    bool
		csvContent    string
		errorContains string
	}{
		{
			name:          "file not found",
			csvFile:       "nonexistent.csv",
			execTemplate:  "echo {{.name}}",
			createFile:    false,
			errorContains: "failed to open data file",
		},
		{
			name:         "invalid template syntax",
			csvFile:      "",
			execTemplate: "echo {{.name",
			createFile:   true,
			csvContent: `user_id,name,email
1,John Doe,john@example.com`,
			errorContains: "failed to parse template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var csvFile string
			
			if tt.createFile {
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
				csvFile = tmpFile.Name()
			} else {
				csvFile = tt.csvFile
			}

			mockExecutor := func(command string) error {
				return nil
			}

			err := processCSVWithExecutor(csvFile, tt.execTemplate, mockExecutor)
			if err == nil {
				t.Fatal("Expected error but got nil")
			}

			if !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("Expected error to contain %q, got: %v", tt.errorContains, err)
			}
		})
	}
}

func TestProcessDataFileWithExecutor(t *testing.T) {
	tests := []struct {
		name             string
		fileName         string
		fileContent      string
		execTemplate     string
		expectedCommands []string
		expectError      bool
	}{
		{
			name:     "JSON file processing",
			fileName: "test.json",
			fileContent: `[
				{"user_id": "1", "name": "John Doe", "email": "john@example.com"},
				{"user_id": "2", "name": "Jane Smith", "email": "jane@example.com"}
			]`,
			execTemplate: "echo {{.name}} {{.email}}",
			expectedCommands: []string{
				"echo John Doe john@example.com",
				"echo Jane Smith jane@example.com",
			},
			expectError: false,
		},
		{
			name:     "JSONL file processing",
			fileName: "test.jsonl",
			fileContent: `{"user_id": "1", "name": "John Doe", "email": "john@example.com"}
{"user_id": "2", "name": "Jane Smith", "email": "jane@example.com"}`,
			execTemplate: "echo {{.name}} {{.email}}",
			expectedCommands: []string{
				"echo John Doe john@example.com",
				"echo Jane Smith jane@example.com",
			},
			expectError: false,
		},
		{
			name:     "CSV fallback for unknown extension",
			fileName: "test.data",
			fileContent: `user_id,name,email
1,John Doe,john@example.com
2,Jane Smith,jane@example.com`,
			execTemplate: "echo {{.name}} {{.email}}",
			expectedCommands: []string{
				"echo John Doe john@example.com",
				"echo Jane Smith jane@example.com",
			},
			expectError: false,
		},
		{
			name:     "JSON with numeric and boolean values",
			fileName: "test.json",
			fileContent: `[
				{"id": 1, "active": true, "score": 95.5, "name": "Alice"},
				{"id": 2, "active": false, "score": 87.2, "name": "Bob"}
			]`,
			execTemplate: "echo {{.name}} {{.id}} {{.active}} {{.score}}",
			expectedCommands: []string{
				"echo Alice 1 true 96",
				"echo Bob 2 false 87",
			},
			expectError: false,
		},
		{
			name:     "CSV with missing column referenced in template",
			fileName: "test.csv",
			fileContent: `name,email
John Doe,john@example.com
Jane Smith,jane@example.com`,
			execTemplate: "echo {{.user_id}} {{.name}} {{.email}}",
			expectedCommands: []string{
				"echo  John Doe john@example.com",
				"echo  Jane Smith jane@example.com",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile, err := os.CreateTemp("", tt.fileName)
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.fileContent); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Track executed commands
			var executedCommands []string
			mockExecutor := func(command string) error {
				executedCommands = append(executedCommands, command)
				return nil
			}

			// Execute test
			err = processDataFileWithExecutor(tmpFile.Name(), tt.execTemplate, mockExecutor)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Fatal("Expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				// Verify executed commands
				if len(executedCommands) != len(tt.expectedCommands) {
					t.Fatalf("Expected %d commands, got %d", len(tt.expectedCommands), len(executedCommands))
				}

				for i, expected := range tt.expectedCommands {
					if executedCommands[i] != expected {
						t.Errorf("Command %d: expected %q, got %q", i, expected, executedCommands[i])
					}
				}
			}
		})
	}
}