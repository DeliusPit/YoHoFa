package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: dbbackup <source.db> <destination.db>")
		os.Exit(2)
	}

	db, err := sql.Open("sqlite", os.Args[1])
	if err != nil {
		fatal(err)
	}
	defer db.Close()

	if err := os.RemoveAll(os.Args[2]); err != nil {
		fatal(err)
	}
	destination := strings.ReplaceAll(os.Args[2], "'", "''")
	if _, err := db.Exec("vacuum into '" + destination + "'"); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
