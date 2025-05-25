# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

xrun is a CLI tool that executes commands against data from CSV, JSON, or JSONL files using Go template substitution. It reads structured data and executes user-defined commands for each row/record with template variable substitution.

## Development Commands

### Building and Testing
- `go build` - Build the binary
- `go test -v` - Run all tests with verbose output
- `go test -v ./...` - Run tests in all packages
- `go test -run TestSpecificFunction` - Run a specific test

### Release Management
- `goreleaser check` - Validate GoReleaser configuration
- `goreleaser release --snapshot --clean` - Test release build locally
- Tag with `v*` format triggers automated release via GitHub Actions

### Installation for Testing
- `go install .` - Install current version locally for testing
- `export PATH=$PATH:$(go env GOPATH)/bin` - Ensure GOPATH/bin is in PATH

## Code Architecture

### Core Data Flow
1. **Config Creation**: User flags → `Config` struct containing all options
2. **File Type Detection**: Extension-based routing to appropriate parser
3. **Data Processing**: File → structured data → template execution → command execution
4. **Progress Tracking**: Each command execution includes progress information

### Key Components

#### Configuration Management
- `Config` struct centralizes all execution parameters (DataFile, Template, DryRun, NoLogFiles, LogWriter)
- `processDataFile(config Config)` is the main entry point that handles log writer setup and command executor creation

#### Command Execution Interface
- `CommandExecutor func(command string, progress Progress) error` - Unified interface for all command execution
- `Progress` struct tracks current position and total count for progress display
- Supports both dry-run (printing) and actual execution with logging

#### Data Format Support
- **CSV**: Headers become template variables, processed via `processCSVWithExecutor`
- **JSON**: Array of objects, keys become template variables via `processJSONWithExecutor`  
- **JSONL**: Line-by-line JSON objects via `processJSONLWithExecutor`
- All formats use same template execution and command runner pipeline

#### Template Processing
- Uses Go's `text/template` package for variable substitution
- Template variables come from CSV headers or JSON object keys
- Error handling continues processing on template failures but logs errors

#### Logging System
- `LogWriter` handles file-based logging with timestamp-based filenames
- Dual output: commands and output go to both console and log files
- Progress information included in log entries: `[current/total] timestamp Executing: command`

### Test Architecture
- **main_test.go**: Core CSV/JSON processing logic with mock executors
- **dry_run_test.go**: Dry-run mode validation 
- **log_file_test.go**: Log file creation and configuration testing
- All tests use `CommandExecutor` function type with `Progress` parameter for consistency

### Key Design Patterns
- **Dependency Injection**: CommandExecutor functions injected for testability
- **Configuration Object**: Single Config struct replaces multiple function parameters
- **Strategy Pattern**: File format detection routes to appropriate processor
- **Template Method**: Common execution flow with format-specific data parsing

## File Processing Flow
1. Config validation and log writer setup
2. File extension detection (`ext := strings.ToLower(filepath.Ext(config.DataFile))`)
3. Format-specific parsing (CSV reader, JSON decoder, or JSONL scanner)
4. Template compilation (`template.New("command").Parse(execTemplate)`)
5. For each data row: template execution → command generation → execution with progress tracking

## Important Implementation Notes
- The codebase was recently refactored to unify `CommandExecutor` and `ProgressAwareCommandExecutor` into a single interface
- Progress tracking requires reading all data first to determine total count
- Command execution errors don't stop processing but are logged to stderr
- Template execution errors skip the problematic row but continue processing
- LogWriter implements `io.Writer` interface for use with `io.MultiWriter`