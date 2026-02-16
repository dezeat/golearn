package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("golearn — MCQ practice tool")
		fmt.Println("Usage: golearn <command>")
		fmt.Println()
		fmt.Println("Commands:")
		fmt.Println("  tui       Launch the interactive TUI")
		fmt.Println("  import    Import a question pack file")
		fmt.Println("  export    Export a topic to a pack file")
		os.Exit(0)
	}

	switch os.Args[1] {
	case "tui":
		fmt.Println("TUI not yet implemented.")
	case "import":
		fmt.Println("Import not yet implemented.")
	case "export":
		fmt.Println("Export not yet implemented.")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
