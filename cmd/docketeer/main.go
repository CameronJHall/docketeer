package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/CameronJHall/docketeer/internal/config"
	"github.com/CameronJHall/docketeer/internal/mcp"
	"github.com/CameronJHall/docketeer/internal/seed"
	"github.com/CameronJHall/docketeer/internal/store"
	"github.com/CameronJHall/docketeer/internal/tui"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	var doSeed bool
	var runMcpMode bool

	flag.BoolVar(&doSeed, "seed", false, "populate the database with fixture data and exit")
	flag.BoolVar(&runMcpMode, "mcp", false, "run as MCP server over stdio")

	flag.Parse()

	// Load config from ~/.config/docketeer/
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "docketeer: load config: %v\n", err)
		os.Exit(1)
	}

	// Ensure config directory exists, prompt for creds if needed
	if err := config.Ensure(); err != nil {
		fmt.Fprintf(os.Stderr, "docketeer: ensure config: %v\n", err)
		os.Exit(1)
	}

	var s store.Store

	// Determine backend: CLI flag > env var > config file > sqlite default
	backEndType := cfg.GetBackend()

	switch backEndType {
	case "postgres":
		connString := cfg.GetPostgresConn()
		if connString == "" {
			fmt.Fprintf(os.Stderr, "docketeer: postgres connection string not configured\n")
			os.Exit(1)
		}
		s, err = store.NewPostgresStore(connString)
		if err != nil {
			fmt.Fprintf(os.Stderr, "docketeer: open postgres: %v\n", err)
			os.Exit(1)
		}

	case "sqlite", "":
		sqlitePath := cfg.GetSQLitePath()
		s, err = store.NewSQLiteStore(sqlitePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "docketeer: open database: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "docketeer: unknown backend: %s\n", backEndType)
		os.Exit(1)
	}

	defer s.Close()

	if doSeed {
		if err := seed.Run(s); err != nil {
			fmt.Fprintf(os.Stderr, "docketeer: seed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("seeded database")
		return
	}

	if runMcpMode {
		server := mcp.NewServer(s)
		if err := server.Run(context.Background(), &mcpsdk.StdioTransport{}); err != nil {
			fmt.Fprintf(os.Stderr, "docketeer: mcp server: %v\n", err)
			os.Exit(1)
		}
		return
	}

	app := tui.New(s)
	p := tea.NewProgram(app)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "docketeer: %v\n", err)
		os.Exit(1)
	}
}
