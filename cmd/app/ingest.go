package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/Rai-Tsumugu/English-Learning/internal/config"
	appdb "github.com/Rai-Tsumugu/English-Learning/internal/db"
	"github.com/Rai-Tsumugu/English-Learning/internal/ingest"

	_ "modernc.org/sqlite"
)

func runIngest(ctx context.Context, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("ingest", flag.ContinueOnError)
	path := fs.String("file", "data/seed/words.sample.json", "words JSON file")
	withEmbed := fs.Bool("embed", false, "(deprecated, ignored) embeddings are not available on the ChatGPT subscription tier")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *withEmbed {
		fmt.Fprintln(os.Stderr, "warning: --embed is ignored: embeddings are not available on the ChatGPT subscription tier")
	}

	d, err := sql.Open(appdb.DriverName, cfg.AppDBPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer d.Close()
	if err := appdb.Migrate(d, "up"); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	opts := ingest.Options{Path: *path}
	ins, upd, emb, err := ingest.Ingest(ctx, d, opts)
	if err != nil {
		return err
	}
	fmt.Printf("ingest: inserted=%d updated=%d embedded=%d\n", ins, upd, emb)
	return nil
}
