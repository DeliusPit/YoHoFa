package store

import (
	"strings"
	"testing"
)

func TestPostgresSchemaRegistersCurrentMigrations(t *testing.T) {
	for _, version := range []string{"010_case_contract_fields", "011_reservations", "012_document_translations", "013_document_analyses_parties", "014_transaction_counterparties"} {
		if !strings.Contains(postgresSchema, "'"+version+"'") {
			t.Errorf("postgres schema does not register %s", version)
		}
	}
	if !strings.Contains(postgresSchema, "'reservation_created'") {
		t.Fatal("postgres schema does not enforce a unique reservation creation event")
	}
}
