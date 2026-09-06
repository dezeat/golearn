// Copyright 2026 dezeat
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/dezeat/golearn/addons/forge/internal/adapters/forgestore"
	forgeapp "github.com/dezeat/golearn/addons/forge/internal/app"
	"github.com/dezeat/golearn/addons/forge/internal/domain"
	corepack "github.com/dezeat/golearn/internal/adapters/pack"
	coresqlite "github.com/dezeat/golearn/internal/adapters/sqlite"
	coreapp "github.com/dezeat/golearn/internal/app"
)

// runDrafts implements the draft screen's three actions from the command line:
// list, add to library, discard.
//
// FORGE.md 3.3's no-junk rule is that unresolved drafts are resolved before new
// generation starts. This is the minimal surface that makes resolving one
// possible; the TUI draft screen is #128's.
func runDrafts(args []string, stdout, stderr io.Writer) int {
	dbPath := coresqlite.DefaultDBPath()
	action, target := "list", int64(0)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--db":
			if i+1 >= len(args) {
				write(stderr, "error: --db requires a value\n")
				return 2
			}
			i++
			dbPath = args[i]
		case "add", "discard", "show":
			action = args[i]
			if i+1 >= len(args) {
				write(stderr, fmt.Sprintf("error: %s requires a draft id\n", action))
				return 2
			}
			i++
			id, err := strconv.ParseInt(args[i], 10, 64)
			if err != nil {
				write(stderr, fmt.Sprintf("error: %q is not a draft id\n", args[i]))
				return 2
			}
			target = id
		case "list":
			action = "list"
		default:
			if strings.HasPrefix(args[i], "-") {
				write(stderr, fmt.Sprintf("error: unknown flag %q\n\n%s", args[i], draftsUsage()))
				return 2
			}
		}
	}

	ctx := context.Background()
	db, err := coresqlite.Open(dbPath)
	if err != nil {
		write(stderr, fmt.Sprintf("error: %v\n", err))
		return 1
	}
	defer func() { _ = db.Close() }()

	store, err := forgestore.New(ctx, db)
	if err != nil {
		write(stderr, fmt.Sprintf("error: %v\n", err))
		return 1
	}

	switch action {
	case "list":
		return listDrafts(ctx, store, stdout, stderr)
	case "show":
		return showDraft(ctx, store, target, stdout, stderr)
	case "add":
		return addDraft(ctx, db, store, target, stdout, stderr)
	case "discard":
		if err := store.DeleteDraft(ctx, target); err != nil {
			write(stderr, fmt.Sprintf("error: %v\n", err))
			return 1
		}
		write(stdout, fmt.Sprintf("Draft %d discarded.\n", target))
		return 0
	default:
		write(stderr, draftsUsage())
		return 2
	}
}

func listDrafts(ctx context.Context, store *forgestore.Store, stdout, stderr io.Writer) int {
	drafts, err := store.ListDrafts(ctx)
	if err != nil {
		write(stderr, fmt.Sprintf("error: %v\n", err))
		return 1
	}
	if len(drafts) == 0 {
		write(stdout, "No unresolved drafts."+"\n")
		return 0
	}
	write(stdout, fmt.Sprintf("%d unresolved draft(s):\n\n", len(drafts)))
	for _, d := range drafts {
		write(stdout, fmt.Sprintf("  [%d] %s\n", d.ID, d.Summary()))
	}
	write(stdout, "\ngolearn-forge drafts show <id> | add <id> | discard <id>"+"\n")
	return 0
}

func showDraft(ctx context.Context, store *forgestore.Store, id int64, stdout, stderr io.Writer) int {
	draft, err := store.GetDraft(ctx, id)
	if err != nil {
		write(stderr, fmt.Sprintf("error: %v\n", err))
		return 1
	}
	write(stdout, fmt.Sprintf("Draft %d — %s\n", draft.ID, draft.Summary()))
	if prov, ok := draft.Provenance(); ok {
		write(stdout, fmt.Sprintf("  generated  %s by %s\n",
			prov.GeneratedAt.Format("2006-01-02 15:04"), prov.Model))
		for _, src := range prov.Sources {
			write(stdout, fmt.Sprintf("  source     %s %s\n", src.ID, src.URL))
		}
	}
	write(stdout, "\n")
	printQuestions(stdout, draft.Pack.Questions)
	return 0
}

func addDraft(ctx context.Context, db *sql.DB, store *forgestore.Store, id int64, stdout, stderr io.Writer) int {
	draft, err := store.GetDraft(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrDraftNotFound) {
			write(stderr, fmt.Sprintf("error: no draft %d\n", id))
			return 1
		}
		write(stderr, fmt.Sprintf("error: %v\n", err))
		return 1
	}

	// Acceptance runs the core's standard atomic import (D-004), the same path
	// a hand-authored file takes — not a private insert.
	acceptor := forgeapp.NewDraftAcceptor(
		coreapp.NewImportService(corepack.NewReader(),
			coresqlite.NewTopicRepo(db), coresqlite.NewQuestionRepo(db)),
		store)

	imported, err := acceptor.Accept(ctx, draft)
	if err != nil {
		write(stderr, fmt.Sprintf("error: %v\n", err))
		return 1
	}
	write(stdout, fmt.Sprintf("Added draft %d to the library: %d inserted, %d already present.\n",
		id, imported.Inserted, imported.Duplicates))
	return 0
}

func draftsUsage() string {
	return `Usage:
  golearn-forge drafts [list]
  golearn-forge drafts show <id>
  golearn-forge drafts add <id>
  golearn-forge drafts discard <id>

A draft is a finished, validated pack that is not yet library content.
Adding one runs the same atomic import a hand-authored pack file takes.
`
}
