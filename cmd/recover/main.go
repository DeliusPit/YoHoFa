package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"hohoho/internal/config"
	"hohoho/internal/systembackup"
)

func main() {
	snapshotID := flag.String("snapshot", "", "ID van de restic-snapshot die wordt hersteld")
	list := flag.Bool("list", false, "toon beschikbare snapshots")
	noSafetyBackup := flag.Bool("no-safety-backup", false, "sla de veiligheidsbackup voor herstel over")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	configuration := config.RuntimeFromEnv()
	if err := configuration.Validate(); err != nil {
		fatal(err)
	}
	service, err := systembackup.New(context.Background(), configuration, logger)
	if err != nil {
		fatal(err)
	}
	if *list {
		snapshots, err := service.Snapshots(context.Background())
		if err != nil {
			fatal(err)
		}
		for _, snapshot := range snapshots {
			fmt.Printf("%s\t%s\n", snapshot.ID, snapshot.Time)
		}
		return
	}
	if *snapshotID == "" {
		fatal(fmt.Errorf("gebruik --snapshot ID of --list"))
	}
	if err := service.RecoverNow(context.Background(), *snapshotID, !*noSafetyBackup); err != nil {
		fatal(err)
	}
	fmt.Printf("Snapshot %s is hersteld. Start de applicatie opnieuw.\n", *snapshotID)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Herstel mislukt:", err)
	os.Exit(1)
}
