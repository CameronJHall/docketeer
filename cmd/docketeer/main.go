package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/CameronJHall/docketeer/internal/seed"
	"github.com/CameronJHall/docketeer/internal/store"
	"github.com/CameronJHall/docketeer/internal/tui"
)

func main() {
	var dbPath string
	var doSeed bool
	flag.StringVar(&dbPath, "db", "", "path to SQLite database file (default: ~/.local/share/docketeer/docketeer.db)")
	flag.BoolVar(&doSeed, "seed", false, "populate the database with fixture data and exit")
	flag.Parse()

	if dbPath == "" {
		var err error
		dbPath, err = store.DefaultDBPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "docketeer: %v\n", err)
			os.Exit(1)
		}
	}

	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docketeer: open database: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	if doSeed {
		if err := seed.Run(s); err != nil {
			fmt.Fprintf(os.Stderr, "docketeer: seed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("seeded database:", dbPath)
		return
	}

	app := tui.New(s)
	p := tea.NewProgram(app)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "docketeer: %v\n", err)
		os.Exit(1)
	}
}
