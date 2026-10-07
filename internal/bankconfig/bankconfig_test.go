package bankconfig

import (
	"strings"
	"testing"
)

const spardaYAML = `bank:
  name: "Sparda-Bank West"

source:
  format: csv
  delimiter: ";"
  encoding: utf-8

mapping:
  transaction_date: Buchungstag
  amount_cents: Betrag
  description: Verwendungszweck
  counterparty: Auftraggeber/Empfänger

formats:
  transaction_date: "02.01.2006"
  decimal_separator: ","
`

func TestParseAndNormalizeBankConfig(t *testing.T) {
	item, err := Parse("sparda", spardaYAML)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Normalize(item, map[string]string{"Buchungstag": "04.10.2026", "Betrag": "-83,42", "Verwendungszweck": "ALDI NORD", "Auftraggeber/Empfänger": "ALDI"})
	if err != nil {
		t.Fatal(err)
	}
	if got.TransactionDate != "2026-10-04" || got.AmountCents != -8342 || got.Description != "ALDI NORD" || got.Counterparty != "ALDI" {
		t.Fatalf("unexpected normalized transaction: %#v", got)
	}
}

func TestPreviewCSVUsesMappingOnly(t *testing.T) {
	item, err := Parse("sparda", spardaYAML)
	if err != nil {
		t.Fatal(err)
	}
	items, err := PreviewCSV(item, strings.NewReader("Buchungstag;Betrag;Verwendungszweck;Auftraggeber/Empfänger\n04.10.2026;12,34;Loon;Werkgever\n"), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].AmountCents != 1234 {
		t.Fatalf("unexpected preview: %#v", items)
	}
}

func TestParseCSVRowsKeepsRawData(t *testing.T) {
	item, err := Parse("sparda", spardaYAML)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ParseCSVRows(item, strings.NewReader("Buchungstag;Betrag;Verwendungszweck;Auftraggeber/Empfänger\n04.10.2026;12,34;Loon;Werkgever\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Row != 2 || rows[0].Raw["Betrag"] != "12,34" || rows[0].Transaction.AmountCents != 1234 {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}

func TestTriodosHeaderlessDebitCreditCSV(t *testing.T) {
	const raw = `bank:
  name: "Triodos Bank"

source:
  format: csv
  delimiter: ","
  encoding: utf-8
  quote: '"'
  header: false

columns:
  transaction_date: 0
  account: 1
  amount: 2
  debit_credit: 3
  counterparty: 4
  counterparty_account: 5
  transaction_code: 6
  description: 7
  balance: 8

formats:
  transaction_date: "02-01-2006"
  decimal_separator: ","
  thousands_separator: "."

amount:
  mode: debit_credit
  debit_value: "Debet"
  credit_value: "Credit"

mapping:
  transaction_date: transaction_date
  amount_cents: amount
  description: description
  counterparty: counterparty
`
	item, err := Parse("triodos", raw)
	if err != nil {
		t.Fatal(err)
	}
	csv := `"01-01-2026","NL18TRIO0254363105","1,04","Debet","","","RE","Debet rente van 01-10-2025 tot en met 31-12-2025","3.643,39"
"02-01-2026","NL18TRIO0254363105","1.783,52","Debet","ELAN WONINGHYPOTHEKEN ON","ABNANL2A NL89ABNA0449424790","EI","NL98ZZZ653562170000 713333 260101-2317-GN307382701-2967 4000000019192485/ovknr 307382701/Hypotrust 202512","1.550,97"
"02-01-2026","NL18TRIO0254363105","15,00","Credit","Nederlandse Loterij Organisatie B.V.","INGBNL2A NL62INGB0663707048","ET","Gefeliciteerd, Gewonnen met Oudejaarstrekking, Trekking 31-12-2025, 322179556.23282","1.565,97"
`
	rows, err := ParseCSVRows(item, strings.NewReader(csv), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[0].Transaction.AmountCents != -104 || rows[1].Transaction.AmountCents != -178352 || rows[2].Transaction.AmountCents != 1500 {
		t.Fatalf("unexpected amounts: %#v", rows)
	}
	if rows[1].Transaction.Counterparty != "ELAN WONINGHYPOTHEKEN ON" || rows[2].Transaction.TransactionDate != "2026-01-02" {
		t.Fatalf("unexpected normalized rows: %#v", rows)
	}
}
