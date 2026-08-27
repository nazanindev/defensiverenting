// runsql executes one SQL script file against DATABASE_URL and prints every
// result set. It exists because the dated, hand-verified surgery scripts in
// scripts/ need a way to run on a machine with no psql installed; the whole
// file is sent as a single simple-protocol exec, so a script that wraps
// itself in BEGIN/COMMIT aborts atomically on the first error.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) != 2 {
		log.Fatal("usage: runsql <file.sql> (connects to $DATABASE_URL)")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	script, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	results, err := conn.PgConn().Exec(ctx, string(script)).ReadAll()
	for _, res := range results {
		if len(res.FieldDescriptions) > 0 {
			for i, fd := range res.FieldDescriptions {
				if i > 0 {
					fmt.Print(" | ")
				}
				fmt.Print(fd.Name)
			}
			fmt.Println()
			for _, row := range res.Rows {
				for i, cell := range row {
					if i > 0 {
						fmt.Print(" | ")
					}
					fmt.Print(string(cell))
				}
				fmt.Println()
			}
		}
		fmt.Println(res.CommandTag)
	}
	if err != nil {
		log.Fatalf("script failed (any open transaction was rolled back): %v", err)
	}
}
