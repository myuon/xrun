package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// CommandExecutor is a function type for executing commands, allowing for dependency injection
type CommandExecutor func(command string) error

// ProgressAwareCommandExecutor is a function type for executing commands with progress information
type ProgressAwareCommandExecutor func(command string, current int, total int) error

func main() {
	var dataFile string
	var execTemplate string
	var dryRun bool

	flag.StringVar(&dataFile, "d", "", "Path to the data file (CSV/JSON/JSONL)")
	flag.StringVar(&execTemplate, "e", "", "Command template to execute for each row")
	flag.BoolVar(&dryRun, "dry-run", false, "Print commands to stdout instead of executing them")
	flag.Parse()

	if dataFile != "" && execTemplate != "" {
		if err := processDataFileWithDryRun(dataFile, execTemplate, dryRun); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> OR %s -d <data-file> -e \"<command-template>\"\n", os.Args[0], os.Args[0])
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "version":
		fmt.Println("xrun v0.1.0")
	case "help":
		showHelp()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		showHelp()
		os.Exit(1)
	}
}

func processDataFile(dataFile, execTemplate string) error {
	return processDataFileWithExecutor(dataFile, execTemplate, executeCommand)
}

func processDataFileWithDryRun(dataFile, execTemplate string, dryRun bool) error {
	var progressExecutor ProgressAwareCommandExecutor
	if dryRun {
		progressExecutor = func(command string, current int, total int) error {
			return printCommand(command)
		}
	} else {
		progressExecutor = executeCommandWithProgress
	}
	
	ext := strings.ToLower(filepath.Ext(dataFile))
	
	switch ext {
	case ".json":
		return processJSONWithProgressExecutor(dataFile, execTemplate, progressExecutor)
	case ".jsonl":
		return processJSONLWithProgressExecutor(dataFile, execTemplate, progressExecutor)
	default:
		// Fallback to CSV for unknown extensions or .csv
		return processCSVWithProgressExecutor(dataFile, execTemplate, progressExecutor)
	}
}

func processDataFileWithExecutor(dataFile, execTemplate string, executor CommandExecutor) error {
	ext := strings.ToLower(filepath.Ext(dataFile))
	
	// Convert regular executor to progress-aware executor
	progressExecutor := func(command string, current int, total int) error {
		return executor(command)
	}
	
	switch ext {
	case ".json":
		return processJSONWithProgressExecutor(dataFile, execTemplate, progressExecutor)
	case ".jsonl":
		return processJSONLWithProgressExecutor(dataFile, execTemplate, progressExecutor)
	default:
		// Fallback to CSV for unknown extensions or .csv
		return processCSVWithProgressExecutor(dataFile, execTemplate, progressExecutor)
	}
}

func processCSV(dataFile, execTemplate string) error {
	return processCSVWithExecutor(dataFile, execTemplate, executeCommand)
}

// processCSVWithExecutor handles CSV processing with an injectable command executor
func processCSVWithExecutor(dataFile, execTemplate string, executor CommandExecutor) error {
	// Convert to progress-aware executor
	progressExecutor := func(command string, current int, total int) error {
		return executor(command)
	}
	return processCSVWithProgressExecutor(dataFile, execTemplate, progressExecutor)
}

// processCSVWithProgressExecutor handles CSV processing with progress tracking
func processCSVWithProgressExecutor(dataFile, execTemplate string, executor ProgressAwareCommandExecutor) error {
	file, err := os.Open(dataFile)
	if err != nil {
		return fmt.Errorf("failed to open data file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	
	headers, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV headers: %v", err)
	}

	// Read all rows first to get total count
	var allRows [][]string
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read CSV row: %v", err)
		}
		allRows = append(allRows, row)
	}

	tmpl, err := template.New("command").Parse(execTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	total := len(allRows)
	for i, row := range allRows {
		data := make(map[string]string)
		for j, header := range headers {
			if j < len(row) {
				data[header] = row[j]
			}
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			fmt.Fprintf(os.Stderr, "Template execution error for row %v: %v\n", row, err)
			continue
		}

		command := buf.String()
		if err := executor(command, i+1, total); err != nil {
			fmt.Fprintf(os.Stderr, "Command execution error: %v\n", err)
		}
	}

	return nil
}

// processJSONWithExecutor handles JSON array processing with an injectable command executor
func processJSONWithExecutor(dataFile, execTemplate string, executor CommandExecutor) error {
	// Convert to progress-aware executor
	progressExecutor := func(command string, current int, total int) error {
		return executor(command)
	}
	return processJSONWithProgressExecutor(dataFile, execTemplate, progressExecutor)
}

// processJSONWithProgressExecutor handles JSON array processing with progress tracking
func processJSONWithProgressExecutor(dataFile, execTemplate string, executor ProgressAwareCommandExecutor) error {
	file, err := os.Open(dataFile)
	if err != nil {
		return fmt.Errorf("failed to open data file: %v", err)
	}
	defer file.Close()

	var data []map[string]any
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return fmt.Errorf("failed to parse JSON: %v", err)
	}

	tmpl, err := template.New("command").Parse(execTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	total := len(data)
	for i, row := range data {
		// Convert interface{} values to strings for template compatibility
		stringRow := make(map[string]string)
		for key, value := range row {
			if value == nil {
				stringRow[key] = ""
			} else {
				switch v := value.(type) {
				case string:
					stringRow[key] = v
				case float64:
					stringRow[key] = fmt.Sprintf("%g", v)
				case bool:
					stringRow[key] = fmt.Sprintf("%t", v)
				default:
					// For complex types, convert to JSON string
					jsonBytes, _ := json.Marshal(v)
					stringRow[key] = string(jsonBytes)
				}
			}
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, stringRow); err != nil {
			fmt.Fprintf(os.Stderr, "Template execution error for row %v: %v\n", row, err)
			continue
		}

		command := buf.String()
		if err := executor(command, i+1, total); err != nil {
			fmt.Fprintf(os.Stderr, "Command execution error: %v\n", err)
		}
	}

	return nil
}

// processJSONLWithExecutor handles JSONL (JSON Lines) processing with an injectable command executor
func processJSONLWithExecutor(dataFile, execTemplate string, executor CommandExecutor) error {
	// Convert to progress-aware executor
	progressExecutor := func(command string, current int, total int) error {
		return executor(command)
	}
	return processJSONLWithProgressExecutor(dataFile, execTemplate, progressExecutor)
}

// processJSONLWithProgressExecutor handles JSONL (JSON Lines) processing with progress tracking
func processJSONLWithProgressExecutor(dataFile, execTemplate string, executor ProgressAwareCommandExecutor) error {
	file, err := os.Open(dataFile)
	if err != nil {
		return fmt.Errorf("failed to open data file: %v", err)
	}
	defer file.Close()

	tmpl, err := template.New("command").Parse(execTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	// Read all lines first to get total count
	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" { // Skip empty lines
			allLines = append(allLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading JSONL file: %v", err)
	}

	total := len(allLines)
	for i, line := range allLines {
		var row map[string]interface{}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse JSON on line %d: %v\n", i+1, err)
			continue
		}

		// Convert interface{} values to strings for template compatibility
		stringRow := make(map[string]string)
		for key, value := range row {
			if value == nil {
				stringRow[key] = ""
			} else {
				switch v := value.(type) {
				case string:
					stringRow[key] = v
				case float64:
					stringRow[key] = fmt.Sprintf("%g", v)
				case bool:
					stringRow[key] = fmt.Sprintf("%t", v)
				default:
					// For complex types, convert to JSON string
					jsonBytes, _ := json.Marshal(v)
					stringRow[key] = string(jsonBytes)
				}
			}
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, stringRow); err != nil {
			fmt.Fprintf(os.Stderr, "Template execution error for line %d: %v\n", i+1, err)
			continue
		}

		command := buf.String()
		if err := executor(command, i+1, total); err != nil {
			fmt.Fprintf(os.Stderr, "Command execution error: %v\n", err)
		}
	}

	return nil
}

func executeCommand(command string) error {
	return executeCommandWithProgress(command, 0, 0)
}

func executeCommandWithProgress(command string, current int, total int) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Format the log with timestamp and progress
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	if current > 0 && total > 0 {
		fmt.Printf("[%d/%d] %s Executing: %s\n", current, total, timestamp, command)
	} else {
		fmt.Printf("%s Executing: %s\n", timestamp, command)
	}
	return cmd.Run()
}

func printCommand(command string) error {
	fmt.Println(command)
	return nil
}

func showHelp() {
	fmt.Println("xrun - CLI tool")
	fmt.Println("\nUsage:")
	fmt.Println("  xrun <command>")
	fmt.Println("  xrun -d <data-file> -e \"<command-template>\" [--dry-run]")
	fmt.Println("\nCommands:")
	fmt.Println("  version    Show version information")
	fmt.Println("  help       Show this help message")
	fmt.Println("\nData processing options:")
	fmt.Println("  -d         Path to the data file (CSV/JSON/JSONL)")
	fmt.Println("  -e         Command template to execute for each row")
	fmt.Println("  --dry-run  Print commands to stdout instead of executing them")
	fmt.Println("\nSupported file formats:")
	fmt.Println("  .csv       CSV files with headers")
	fmt.Println("  .json      JSON array of objects")
	fmt.Println("  .jsonl     JSON Lines (one JSON object per line)")
	fmt.Println("  other      Defaults to CSV parsing")
	fmt.Println("\nTemplate syntax:")
	fmt.Println("  Use {{.field_name}} to substitute values from data fields")
}