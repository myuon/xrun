package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <command>\n", os.Args[0])
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

func showHelp() {
	fmt.Println("xrun - CLI tool")
	fmt.Println("\nUsage:")
	fmt.Println("  xrun <command>")
	fmt.Println("\nCommands:")
	fmt.Println("  version    Show version information")
	fmt.Println("  help       Show this help message")
}