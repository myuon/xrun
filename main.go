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

// LogWriter handles writing to log files
type LogWriter struct {
	file *os.File
}

func (lw *LogWriter) Write(p []byte) (n int, err error) {
	if lw.file != nil {
		return lw.file.Write(p)
	}
	return len(p), nil
}

func (lw *LogWriter) Close() error {
	if lw.file != nil {
		return lw.file.Close()
	}
	return nil
}

func main() {
	var dataFile string
	var execTemplate string
	var dryRun bool
	var noLogFiles bool

	flag.StringVar(&dataFile, "d", "", "Path to the data file (CSV/JSON/JSONL)")
	flag.StringVar(&execTemplate, "e", "", "Command template to execute for each row")
	flag.BoolVar(&dryRun, "dry-run", false, "Print commands to stdout instead of executing them")
	flag.BoolVar(&noLogFiles, "no-log-files", false, "Skip logging execution output to files")
	flag.Parse()

	if dataFile != "" && execTemplate != "" {
		if err := processDataFileWithOptions(dataFile, execTemplate, dryRun, noLogFiles); err != nil {
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

func createLogWriter(dataFile string) (*LogWriter, error) {
	// Extract filename without extension for log file naming
	baseName := filepath.Base(dataFile)
	ext := filepath.Ext(baseName)
	if ext != "" {
		baseName = baseName[:len(baseName)-len(ext)]
	}
	
	// Create log file name with timestamp
	timestamp := time.Now().Format("20060102-150405")
	logFileName := fmt.Sprintf("xrun-%s-%s.logs", baseName, timestamp)
	
	file, err := os.Create(logFileName)
	if err != nil {
		return nil, err
	}
	
	return &LogWriter{file: file}, nil
}

func createCommandExecutor(dryRun bool, logWriter *LogWriter) CommandExecutor {
	if dryRun {
		return printCommand
	}
	return func(command string) error {
		return executeCommandWithLogging(command, logWriter)
	}
}

func processDataFile(dataFile, execTemplate string) error {
	return processDataFileWithExecutor(dataFile, execTemplate, executeCommand)
}

func processDataFileWithDryRun(dataFile, execTemplate string, dryRun bool) error {
	return processDataFileWithOptions(dataFile, execTemplate, dryRun, false)
}

func processDataFileWithOptions(dataFile, execTemplate string, dryRun, noLogFiles bool) error {
	var logWriter *LogWriter
	var err error
	
	if !dryRun && !noLogFiles {
		logWriter, err = createLogWriter(dataFile)
		if err != nil {
			return fmt.Errorf("failed to create log file: %v", err)
		}
		defer logWriter.Close()
	}
	
	executor := createCommandExecutor(dryRun, logWriter)
	return processDataFileWithExecutor(dataFile, execTemplate, executor)
}

func processDataFileWithExecutor(dataFile, execTemplate string, executor CommandExecutor) error {
	ext := strings.ToLower(filepath.Ext(dataFile))
	
	switch ext {
	case ".json":
		return processJSONWithExecutor(dataFile, execTemplate, executor)
	case ".jsonl":
		return processJSONLWithExecutor(dataFile, execTemplate, executor)
	default:
		// Fallback to CSV for unknown extensions or .csv
		return processCSVWithExecutor(dataFile, execTemplate, executor)
	}
}

func processCSV(dataFile, execTemplate string) error {
	return processCSVWithExecutor(dataFile, execTemplate, executeCommand)
}

// processCSVWithExecutor handles CSV processing with an injectable command executor
func processCSVWithExecutor(dataFile, execTemplate string, executor CommandExecutor) error {
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

	tmpl, err := template.New("command").Parse(execTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read CSV row: %v", err)
		}

		data := make(map[string]string)
		for i, header := range headers {
			if i < len(row) {
				data[header] = row[i]
			}
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			fmt.Fprintf(os.Stderr, "Template execution error for row %v: %v\n", row, err)
			continue
		}

		command := buf.String()
		if err := executor(command); err != nil {
			fmt.Fprintf(os.Stderr, "Command execution error: %v\n", err)
		}
	}

	return nil
}

// processJSONWithExecutor handles JSON array processing with an injectable command executor
func processJSONWithExecutor(dataFile, execTemplate string, executor CommandExecutor) error {
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

	for _, row := range data {
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
		if err := executor(command); err != nil {
			fmt.Fprintf(os.Stderr, "Command execution error: %v\n", err)
		}
	}

	return nil
}

// processJSONLWithExecutor handles JSONL (JSON Lines) processing with an injectable command executor
func processJSONLWithExecutor(dataFile, execTemplate string, executor CommandExecutor) error {
	file, err := os.Open(dataFile)
	if err != nil {
		return fmt.Errorf("failed to open data file: %v", err)
	}
	defer file.Close()

	tmpl, err := template.New("command").Parse(execTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %v", err)
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue // Skip empty lines
		}

		var row map[string]interface{}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse JSON on line %d: %v\n", lineNum, err)
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
			fmt.Fprintf(os.Stderr, "Template execution error for line %d: %v\n", lineNum, err)
			continue
		}

		command := buf.String()
		if err := executor(command); err != nil {
			fmt.Fprintf(os.Stderr, "Command execution error: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading JSONL file: %v", err)
	}

	return nil
}

func executeCommand(command string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	fmt.Printf("Executing: %s\n", command)
	return cmd.Run()
}

func executeCommandWithLogging(command string, logWriter *LogWriter) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	
	// Create multi-writers to output to both console and log file
	var stdoutWriter, stderrWriter io.Writer
	if logWriter != nil {
		stdoutWriter = io.MultiWriter(os.Stdout, logWriter)
		stderrWriter = io.MultiWriter(os.Stderr, logWriter)
		
		// Write command to log file
		fmt.Fprintf(logWriter, "Executing: %s\n", command)
	} else {
		stdoutWriter = os.Stdout
		stderrWriter = os.Stderr
	}
	
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	
	fmt.Printf("Executing: %s\n", command)
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
	fmt.Println("  xrun -d <data-file> -e \"<command-template>\" [--dry-run] [--no-log-files]")
	fmt.Println("\nCommands:")
	fmt.Println("  version    Show version information")
	fmt.Println("  help       Show this help message")
	fmt.Println("\nData processing options:")
	fmt.Println("  -d              Path to the data file (CSV/JSON/JSONL)")
	fmt.Println("  -e              Command template to execute for each row")
	fmt.Println("  --dry-run       Print commands to stdout instead of executing them")
	fmt.Println("  --no-log-files  Skip logging execution output to files")
	fmt.Println("\nSupported file formats:")
	fmt.Println("  .csv       CSV files with headers")
	fmt.Println("  .json      JSON array of objects")
	fmt.Println("  .jsonl     JSON Lines (one JSON object per line)")
	fmt.Println("  other      Defaults to CSV parsing")
	fmt.Println("\nTemplate syntax:")
	fmt.Println("  Use {{.field_name}} to substitute values from data fields")
	fmt.Println("\nLog files:")
	fmt.Println("  By default, execution output is saved to xrun-[data-file-name]-[timestamp].logs")
}