package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"hohoho/internal/store"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: migrate-sqlite-postgres <source-sqlite.db> <postgres-url> (core data only; refuses domain data)")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := store.MigrateSQLiteToPostgres(ctx, os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("Migratie voltooid: households=%d accounts=%d main_categories=%d categories=%d transactions=%d\n", result.Households, result.Accounts, result.MainCategories, result.Categories, result.Transactions)
}
