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

// Progress represents the current execution progress
type Progress struct {
	Current int
	Total   int
}

// CommandExecutor is a function type for executing commands with progress information
type CommandExecutor func(command string, progress Progress) error

// Config holds the configuration for processing data files
type Config struct {
	DataFile     string
	Template     string
	DryRun       bool
	NoLogFiles   bool
	LogWriter    *LogWriter
}

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
	var inputFile string
	var dryRun bool
	var noLogFiles bool

	flag.StringVar(&dataFile, "d", "", "Path to the data file (CSV/JSON/JSONL)")
	flag.StringVar(&execTemplate, "e", "", "Command template to execute for each row")
	flag.StringVar(&inputFile, "i", "", "Path to file containing command template")
	flag.BoolVar(&dryRun, "dry-run", false, "Print commands to stdout instead of executing them")
	flag.BoolVar(&noLogFiles, "no-log-files", false, "Skip logging execution output to files")
	flag.Parse()

	// Validate mutual exclusivity of -e and -i flags
	if execTemplate != "" && inputFile != "" {
		fmt.Fprintf(os.Stderr, "Error: -e and -i flags are mutually exclusive\n")
		os.Exit(1)
	}

	var template string
	if inputFile != "" {
		// Read template from file
		content, err := os.ReadFile(inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input file: %v\n", err)
			os.Exit(1)
		}
		template = strings.TrimSpace(string(content))
	} else {
		template = execTemplate
	}

	if dataFile != "" && template != "" {
		config := Config{
			DataFile:   dataFile,
			Template:   template,
			DryRun:     dryRun,
			NoLogFiles: noLogFiles,
		}
		if err := processDataFile(config); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command> OR %s -d <data-file> (-e \"<command-template>\" | -i <input-file>)\n", os.Args[0], os.Args[0])
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

func processDataFile(config Config) error {
	// Set up log writer if needed
	if !config.DryRun && !config.NoLogFiles {
		logWriter, err := createLogWriter(config.DataFile)
		if err != nil {
			return fmt.Errorf("failed to create log file: %v", err)
		}
		defer logWriter.Close()
		config.LogWriter = logWriter
	}
	
	// Create command executor
	var executor CommandExecutor
	if config.DryRun {
		executor = func(command string, progress Progress) error {
			return printCommand(command)
		}
	} else {
		executor = func(command string, progress Progress) error {
			return executeCommandWithProgressAndLogging(command, progress.Current, progress.Total, config.LogWriter)
		}
	}
	
	ext := strings.ToLower(filepath.Ext(config.DataFile))
	
	switch ext {
	case ".json":
		return processJSONWithExecutor(config.DataFile, config.Template, executor)
	case ".jsonl":
		return processJSONLWithExecutor(config.DataFile, config.Template, executor)
	default:
		// Fallback to CSV for unknown extensions or .csv
		return processCSVWithExecutor(config.DataFile, config.Template, executor)
	}
}

func processDataFileWithDryRun(dataFile, execTemplate string, dryRun bool) error {
	config := Config{
		DataFile:   dataFile,
		Template:   execTemplate,
		DryRun:     dryRun,
		NoLogFiles: false,
	}
	return processDataFile(config)
}

func processDataFileWithOptions(dataFile, execTemplate string, dryRun, noLogFiles bool) error {
	config := Config{
		DataFile:   dataFile,
		Template:   execTemplate,
		DryRun:     dryRun,
		NoLogFiles: noLogFiles,
	}
	return processDataFile(config)
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
		progress := Progress{Current: i + 1, Total: total}
		if err := executor(command, progress); err != nil {
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
		progress := Progress{Current: i + 1, Total: total}
		if err := executor(command, progress); err != nil {
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
		var row map[string]any
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
		progress := Progress{Current: i + 1, Total: total}
		if err := executor(command, progress); err != nil {
			fmt.Fprintf(os.Stderr, "Command execution error: %v\n", err)
		}
	}

	return nil
}

func executeCommand(command string, progress Progress) error {
	return executeCommandWithProgressAndLogging(command, progress.Current, progress.Total, nil)
}


func executeCommandWithProgressAndLogging(command string, current int, total int, logWriter *LogWriter) error {
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("empty command")
	}

	cmd := exec.Command("bash", "-c", command)
	
	// Create multi-writers to output to both console and log file
	var stdoutWriter, stderrWriter io.Writer
	if logWriter != nil {
		stdoutWriter = io.MultiWriter(os.Stdout, logWriter)
		stderrWriter = io.MultiWriter(os.Stderr, logWriter)
	} else {
		stdoutWriter = os.Stdout
		stderrWriter = os.Stderr
	}
	
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderrWriter
	
	// Format the log with timestamp and progress
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	var logMessage string
	if current > 0 && total > 0 {
		logMessage = fmt.Sprintf("[%d/%d] %s Executing: %s", current, total, timestamp, command)
	} else {
		logMessage = fmt.Sprintf("%s Executing: %s", timestamp, command)
	}
	
	fmt.Println(logMessage)
	if logWriter != nil {
		fmt.Fprintln(logWriter, logMessage)
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
	fmt.Println("  xrun -d <data-file> (-e \"<command-template>\" | -i <input-file>) [--dry-run] [--no-log-files]")
	fmt.Println("\nCommands:")
	fmt.Println("  version    Show version information")
	fmt.Println("  help       Show this help message")
	fmt.Println("\nData processing options:")
	fmt.Println("  -d              Path to the data file (CSV/JSON/JSONL)")
	fmt.Println("  -e              Command template to execute for each row")
	fmt.Println("  -i              Path to file containing command template")
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