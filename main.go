package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"text/template"
)

func main() {
	var dataFile string
	var execTemplate string

	flag.StringVar(&dataFile, "d", "", "Path to the data file (CSV)")
	flag.StringVar(&execTemplate, "e", "", "Command template to execute for each row")
	flag.Parse()

	if dataFile != "" && execTemplate != "" {
		if err := processCSV(dataFile, execTemplate); err != nil {
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

func processCSV(dataFile, execTemplate string) error {
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
		if err := executeCommand(command); err != nil {
			fmt.Fprintf(os.Stderr, "Command execution error: %v\n", err)
		}
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

func showHelp() {
	fmt.Println("xrun - CLI tool")
	fmt.Println("\nUsage:")
	fmt.Println("  xrun <command>")
	fmt.Println("  xrun -d <data-file> -e \"<command-template>\"")
	fmt.Println("\nCommands:")
	fmt.Println("  version    Show version information")
	fmt.Println("  help       Show this help message")
	fmt.Println("\nData processing options:")
	fmt.Println("  -d         Path to the data file (CSV)")
	fmt.Println("  -e         Command template to execute for each row")
	fmt.Println("\nTemplate syntax:")
	fmt.Println("  Use {{.field_name}} to substitute values from CSV columns")
}