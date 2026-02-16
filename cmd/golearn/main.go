package main

import (
	"fmt"
	"os"

	"github.com/dezeat/golearn/internal/adapters/pack"
	"github.com/dezeat/golearn/internal/adapters/sqlite"
	"github.com/dezeat/golearn/internal/app"
)

func main() {
	dbPath := sqlite.DefaultDBPath()

	// Parse global flags and find the subcommand manually.
	args := os.Args[1:]
	var subcommand string
	var subArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--db" || arg == "-db" {
			if i+1 < len(args) {
				dbPath = args[i+1]
				i++ // skip the value
			}
			continue
		}
		if len(arg) > 5 && arg[:5] == "--db=" {
			dbPath = arg[5:]
			continue
		}
		if arg[0] == '-' {
			continue // skip unknown flags
		}
		// First non-flag argument is the subcommand.
		subcommand = arg
		subArgs = args[i+1:]
		break
	}

	if subcommand == "" {
		printUsage()
		os.Exit(0)
	}

	switch subcommand {
	case "import":
		if err := runImport(dbPath, subArgs); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "tui":
		fmt.Println("TUI not yet implemented.")
	case "export":
		fmt.Println("Export not yet implemented.")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("golearn — MCQ practice tool")
	fmt.Println()
	fmt.Println("Usage: golearn [--db <path>] <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  import <path>   Import a question pack file or directory")
	fmt.Println("  tui             Launch the interactive TUI")
	fmt.Println("  export          Export a topic to a pack file")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Printf("  --db <path>     SQLite database path (default: %s)\n", sqlite.DefaultDBPath())
}

func runImport(dbPath string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: golearn import <file-or-directory>")
	}
	path := args[0]

	db, err := sqlite.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	reader := pack.NewReader()
	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)

	svc := app.NewImportService(reader, topicRepo, questionRepo)
	result, err := svc.Import(path)

	// Print summary even if there was an error (partial results).
	fmt.Println("Import Summary")
	fmt.Println("──────────────")
	if result != nil {
		fmt.Printf("  Files processed: %d\n", result.FilesProcessed)
		fmt.Printf("  Questions added: %d\n", result.Inserted)
		fmt.Printf("  Duplicates skipped: %d\n", result.Duplicates)
		if result.Invalid > 0 {
			fmt.Printf("  Validation errors: %d\n", result.Invalid)
		}
		if len(result.Errors) > 0 {
			fmt.Println("  Errors:")
			for _, e := range result.Errors {
				fmt.Printf("    - %s\n", e)
			}
		}
	}

	return err
}
