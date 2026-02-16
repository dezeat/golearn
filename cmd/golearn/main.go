package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dezeat/golearn/internal/adapters/pack"
	"github.com/dezeat/golearn/internal/adapters/sqlite"
	"github.com/dezeat/golearn/internal/adapters/tui"
	"github.com/dezeat/golearn/internal/app"
)

func main() {
	dbPath := sqlite.DefaultDBPath()

	// Parse global flags and find the subcommand.
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
		if arg == "--help" || arg == "-h" || arg == "help" {
			printUsage()
			os.Exit(0)
		}
		if len(arg) > 0 && arg[0] == '-' {
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
	case "run":
		if err := runSession(dbPath, subArgs); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "tui":
		if err := runTUI(dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "export":
		if err := runExport(dbPath, subArgs); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("golearn — local-first MCQ practice tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  golearn [--db <path>] <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  import <path>                     Import a question pack file or directory")
	fmt.Println("  tui                               Launch the interactive TUI")
	fmt.Println("  run <topic-slug> [--n N]          Start a practice session (text mode)")
	fmt.Println("  export <slug> --out <path>        Export a topic to a pack file")
	fmt.Println("  help                              Show this help message")
	fmt.Println()
	fmt.Println("Global Flags:")
	fmt.Printf("  --db <path>    SQLite database path (default: %s)\n", sqlite.DefaultDBPath())
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  golearn import examples/databricks-pde.yaml")
	fmt.Println("  golearn tui")
	fmt.Println("  golearn run databricks-pde --n 15")
	fmt.Println("  golearn export databricks-pde --out backup.yaml")
	fmt.Println("  golearn export mvp-basics --out out.json --format json")
	fmt.Println()
	fmt.Println("Pack files use YAML or JSON format. See doc/PROJECT.md for the schema.")
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
		fmt.Printf("  Files processed:    %d\n", result.FilesProcessed)
		fmt.Printf("  Questions added:    %d\n", result.Inserted)
		fmt.Printf("  Duplicates skipped: %d\n", result.Duplicates)
		if result.Invalid > 0 {
			fmt.Printf("  Validation errors:  %d\n", result.Invalid)
		}
		if len(result.Errors) > 0 {
			fmt.Println()
			fmt.Println("  Errors:")
			for _, e := range result.Errors {
				fmt.Fprintf(os.Stderr, "    ✗ %s\n", e)
			}
		}
	}

	return err
}

// runTUI launches the Bubble Tea interactive terminal UI.
func runTUI(dbPath string) error {
	db, err := sqlite.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	return tui.Run(db)
}

// runExport implements the `golearn export <topic-slug> --out <path> [--format yaml|json]` command.
func runExport(dbPath string, args []string) error {
	var topicSlug, outPath, format string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--out" || arg == "-out" || arg == "-o":
			if i+1 < len(args) {
				outPath = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--out="):
			outPath = arg[6:]
		case arg == "--format" || arg == "-format" || arg == "-f":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--format="):
			format = arg[9:]
		case len(arg) > 0 && arg[0] == '-':
			// skip unknown flags
		default:
			if topicSlug == "" {
				topicSlug = arg
			}
		}
	}

	if topicSlug == "" {
		return fmt.Errorf("usage: golearn export <topic-slug> --out <path> [--format yaml|json]")
	}
	if outPath == "" {
		return fmt.Errorf("--out <path> is required\n\nUsage: golearn export <topic-slug> --out <path> [--format yaml|json]")
	}

	// Infer format from file extension if not specified.
	if format == "" {
		if strings.HasSuffix(strings.ToLower(outPath), ".json") {
			format = "json"
		} else {
			format = "yaml"
		}
	}

	db, err := sqlite.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)

	svc := app.NewExportService(topicRepo, questionRepo)
	if err := svc.Export(topicSlug, outPath, format); err != nil {
		return err
	}

	fmt.Printf("Exported topic %q to %s\n", topicSlug, outPath)
	return nil
}

// runSession implements the `golearn run <topic-slug> [--n N]` command.
func runSession(dbPath string, args []string) error {
	// Parse: <topic-slug> [--n N]
	var topicSlug string
	n := 10 // default

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--n" || arg == "-n" {
			if i+1 < len(args) {
				val, err := strconv.Atoi(args[i+1])
				if err != nil {
					return fmt.Errorf("invalid --n value %q: %w", args[i+1], err)
				}
				n = val
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--n=") {
			val, err := strconv.Atoi(arg[4:])
			if err != nil {
				return fmt.Errorf("invalid --n value %q: %w", arg[4:], err)
			}
			n = val
			continue
		}
		if topicSlug == "" {
			topicSlug = arg
		}
	}

	if topicSlug == "" {
		return fmt.Errorf("usage: golearn run <topic-slug> [--n N]")
	}

	db, err := sqlite.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	topicRepo := sqlite.NewTopicRepo(db)
	questionRepo := sqlite.NewQuestionRepo(db)
	sessionRepo := sqlite.NewSessionRepo(db)
	attemptRepo := sqlite.NewAttemptRepo(db)

	engine := app.NewSessionEngine(topicRepo, questionRepo, sessionRepo, attemptRepo, nil)

	sessionID, err := engine.StartSession(topicSlug, n, "practice")
	if err != nil {
		return fmt.Errorf("start session: %w", err)
	}

	fmt.Printf("Session %d started (topic: %s, questions: %d)\n", sessionID, topicSlug, engine.QueueLength())
	fmt.Println("Enter choice IDs (e.g. A,C), 's' to skip, 'q' to quit.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	answered := 0
	correctCount := 0
	questionNum := 0

	for {
		q := engine.GetNextQuestion()
		if q == nil {
			break
		}
		questionNum++

		// Print intro if present.
		if q.Intro != "" {
			fmt.Printf("[Context] %s\n\n", q.Intro)
		}

		// Print prompt.
		fmt.Printf("Q%d/%d: %s\n", questionNum, engine.QueueLength(), q.Prompt)

		// Print choices.
		for _, c := range q.Choices {
			fmt.Printf("  %s) %s\n", c.ID, c.Text)
		}
		fmt.Print("\nYour answer: ")

		start := time.Now()
		if !scanner.Scan() {
			break // EOF
		}
		latencyMs := int(time.Since(start).Milliseconds())
		input := strings.TrimSpace(scanner.Text())

		if strings.ToLower(input) == "q" {
			fmt.Println("Quitting session early.")
			break
		}

		skipped := strings.ToLower(input) == "s"
		var selectedIDs []string
		if !skipped {
			parts := strings.Split(input, ",")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					selectedIDs = append(selectedIDs, p)
				}
			}
		}

		correct, err := engine.RecordAttempt(q.ID, selectedIDs, skipped, latencyMs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: failed to record attempt: %v\n", err)
			continue
		}

		answered++
		if skipped {
			fmt.Println("  -> Skipped")
		} else if correct {
			correctCount++
			fmt.Println("  -> Correct!")
		} else {
			fmt.Printf("  -> Incorrect. Correct answer: %s\n", strings.Join(q.CorrectChoiceIDs, ", "))
		}
		fmt.Println()
	}

	// End session.
	if err := engine.EndSession(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to end session: %v\n", err)
	}

	// Print summary.
	fmt.Println()
	fmt.Println("Session Summary")
	fmt.Println("───────────────")
	fmt.Printf("  Total answered: %d\n", answered)
	fmt.Printf("  Correct:        %d\n", correctCount)
	if answered > 0 {
		pct := float64(correctCount) / float64(answered) * 100
		fmt.Printf("  Accuracy:       %.1f%%\n", pct)
	}

	return nil
}
