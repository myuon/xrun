# xrun

A CLI tool written in Go that executes commands against data from CSV or JSON files. xrun takes data files and command templates, then executes the commands for each row of data with template substitution.

## Overview

xrun is designed to streamline batch operations by combining structured data with command execution. It reads data from CSV or JSON files and executes user-defined commands with data from each row substituted into the command template.

## Features

- **Multiple data formats**: Supports CSV and JSON input files
- **Template substitution**: Uses Go template syntax to substitute data values into commands
- **Batch execution**: Automatically processes all rows in the data file
- **Simple CLI interface**: Easy-to-use command-line interface

## Installation

### From source

```bash
git clone https://github.com/myuon/xrun.git
cd xrun
go build -o xrun main.go
```

### Using go install

```bash
go install github.com/myuon/xrun@latest
```

## Usage

```bash
xrun -d <data-file> -e "<command-template>"
```

### Options

- `-d, --data`: Path to the data file (CSV or JSON)
- `-e, --exec`: Command template to execute for each row
- `--dry-run`: Print commands to stdout instead of executing them
- `--no-log-files`: Skip logging execution output to files

### Template Syntax

Use Go template syntax to reference data fields:
- `{{.field_name}}` - Substitute the value of `field_name` from the current row
- Templates support all standard Go template functions

## Examples

### CSV Example

Given a CSV file `users.csv`:
```csv
user_id,name,email
1,John Doe,john@example.com
2,Jane Smith,jane@example.com
3,Bob Johnson,bob@example.com
```

Execute API calls for each user:
```bash
xrun -d users.csv -e "curl -X GET http://api.example.com/users/{{.user_id}}"
```

### JSON Example with POST data

Given a JSON file `data.json`:
```json
[
  {"user_id": "1", "data": "{\"name\":\"John\",\"age\":30}"},
  {"user_id": "2", "data": "{\"name\":\"Jane\",\"age\":25}"}
]
```

Execute API calls with POST data:
```bash
xrun -d data.json -e "curl -i http://api.example.com/users/{{.user_id}} -d '{{.data}}' -H 'Content-Type: application/json'"
```

### File operations

Process files based on CSV data:
```bash
xrun -d files.csv -e "cp {{.source}} {{.destination}}"
```

### Database operations

Execute database queries:
```bash
xrun -d queries.csv -e "mysql -u root -p database -e 'UPDATE users SET status=\"{{.status}}\" WHERE id={{.id}};'"
```

## Dry-Run Mode

Use the `--dry-run` flag to preview commands without executing them. This is useful for:
- Testing command templates before execution
- Debugging template syntax
- Reviewing batch operations safely

### Example

Preview commands that would be executed:
```bash
xrun -d users.csv -e "curl -X GET http://api.example.com/users/{{.user_id}}" --dry-run
```

Output:
```bash
curl -X GET http://api.example.com/users/1
curl -X GET http://api.example.com/users/2
curl -X GET http://api.example.com/users/3
```

## Execution Logging

By default, xrun automatically captures all stdout and stderr output from executed commands to log files. Log files are created in the current directory with the naming format:

```
xrun-[data-file-name]-[timestamp].logs
```

For example, running `xrun -d users.csv -e "..."` creates a log file like `xrun-users-20231025-143022.logs`.

### Logging Options

- **Default behavior**: All command output is logged to files while still being displayed in the console
- **Skip logging**: Use `--no-log-files` to disable log file creation
- **Dry run mode**: No log files are created when using `--dry-run`

### Log File Contents

Log files contain:
- The executed command for each row
- All stdout output from the commands
- All stderr output from the commands
- Timestamps and execution details

## Data File Formats

### CSV Format
- First row should contain column headers
- Headers become template variable names
- Standard CSV parsing rules apply

### JSON Format
- Should contain an array of objects
- Object keys become template variable names
- Supports nested objects (access with dot notation)

## Commands

### Built-in Commands

```bash
xrun version    # Show version information
xrun help       # Show help message
```

## Error Handling

- Invalid data files will cause the program to exit with an error
- Template parsing errors are reported with line numbers
- Command execution errors are logged but don't stop processing of remaining rows

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Version

Current version: v0.1.0