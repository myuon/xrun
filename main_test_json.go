package main

import (
	"os"
	"testing"
)

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