// Command mcp serves the Defensive Renting drafting toolbelt over MCP (stdio),
// so a headless Claude agent (or Claude Code) can research sources and write
// draft playbooks. Reads DATABASE_URL like the other binaries; logs to stderr
// only, because stdout is the MCP protocol channel.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nazanindev/defensiverenting/internal/mcpserver"
	"github.com/nazanindev/defensiverenting/internal/store"
)

func main() {
	log.SetOutput(os.Stderr)
	log.SetFlags(0)

	dsn := flag.String("db", os.Getenv("DATABASE_URL"), "Postgres DSN")
	flag.Parse()
	if *dsn == "" {
		log.Fatal("mcp: DATABASE_URL (or -db) is required")
	}

	ctx := context.Background()
	pg, err := store.New(ctx, *dsn)
	if err != nil {
		log.Fatalf("mcp: connect db: %v", err)
	}
	defer pg.Close()

	srv := mcpserver.New(pg)
	log.Println("mcp: serving drafting toolbelt over stdio")
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("mcp: server exited: %v", err)
	}
}
