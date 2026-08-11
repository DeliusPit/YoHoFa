package bankimport

import (
	"testing"
	"time"

	"github.com/ledongthuc/pdf"
)

func TestParseICSText(t *testing.T) {
	text := `
Datum ICS-klantnummer Volgnummer Bladnummer
7 augustus 2026 18709210027 8 1 van 1
Datum transactie Datum boeking Omschrijving Bedrag in euro's
14 jul. 14 jul. GEINCASSEERD VORIG SALDO 696,75 Bij
07 jul. 08 jul. AMAZON PRIM* 7O3QF89T5 LUXEMBOURG LU 8,99 Af
11 jul. 12 jul. AMZN Mktp DE*458WN44F5 AMZN.COM/BILL LU 69,87 Af
11 jul. 12 jul. AMZN Mktp DE AMZN.COM/BILL LU 24,34 Bij
13 jul. 14 jul. AMZN Mktp DE*L69H46WA5 AMZN.COM/BILL LU 7,98 Af
16 jul. 18 jul. PAYPAL *ALIPAY EUR 17280818884 LU 39,37 Af
17 jul. 18 jul. AmznPrimeNL*Q642751Z5 www.amazon.nl LU 4,99 Af
18 jul. 19 jul. Floorplanner Plus monthly ROTTERDAM NL 4,84 Af
19 jul. 20 jul. AMZN Mktp DE*E35RB84E5 AMZN.COM/BILL LU 340,39 Af
20 jul. 21 jul. AMZN Mktp DE*C30KV6S25 AMZN.COM/BILL LU 520,20 Af
25 jul. 26 jul. Starlink Europe IE 55,00 Af
31 jul. 31 jul. NETFLIX INTERNATIONAL B.V Amsterdam NL 15,99 Af
31 jul. 01 aug. Amazon.de*WJ40Y5TW5 AMAZON.DE LU 49,29 Af
01 aug. 02 aug. CONTABO* PAYMENT MUNCHEN DE 17,85 Af
02 aug. 02 aug. Amazon.de*NG8BA12S4 AMAZON.DE LU 77,14 Af
05 aug. 06 aug. OPENAI *CHATGPT SUBSCR DUBLIN IE 23,00 Af
05 aug. 06 aug. AMZN Mktp DE*4B74L3SF5 AMZN.COM/BILL LU 49,90 Af
`

	preview, err := ParseICSText(text)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Profile != "ics" {
		t.Fatalf("profile = %q, want ics", preview.Profile)
	}
	if len(preview.Transactions) != 17 {
		t.Fatalf("got %d transactions, want 17", len(preview.Transactions))
	}
	if preview.Transactions[0].BookedOn != "2026-07-14" || preview.Transactions[0].AmountCents != 69675 {
		t.Fatalf("unexpected first transaction: %+v", preview.Transactions[0])
	}
	if preview.Transactions[3].AmountCents != 2434 {
		t.Fatalf("refund amount = %d, want 2434", preview.Transactions[3].AmountCents)
	}

	var total int64
	for _, transaction := range preview.Transactions {
		total += transaction.AmountCents
	}
	if total != -56371 {
		t.Fatalf("total = %d, want -56371", total)
	}
}

func TestParseICSTextUsesPreviousYearForDecemberOnJanuaryStatement(t *testing.T) {
	preview, err := ParseICSText("2 januari 2026\n30 dec. 31 dec. TEST BETALING 10,00 Af")
	if err != nil {
		t.Fatal(err)
	}
	if preview.Transactions[0].BookedOn != "2025-12-31" {
		t.Fatalf("bookedOn = %s, want 2025-12-31", preview.Transactions[0].BookedOn)
	}
}

func TestParseICSPDFRowUsesVisualColumns(t *testing.T) {
	statementDate := time.Date(2026, time.April, 7, 0, 0, 0, 0, time.Local)
	row := pdf.TextHorizontal{
		{X: 60, S: "09 mrt."},
		{X: 103, S: "10 mrt."},
		{X: 152, S: "AMZN Mktp DE*HP1IF5XT5"},
		{X: 284, S: "AMZN.COM/BILL"},
		{X: 369, S: "LU"},
		{X: 489, S: "388,99"},
		{X: 535, S: "Af"},
	}

	transaction, ok := parseICSPDFRow(row, statementDate)
	if !ok {
		t.Fatal("row was not recognized")
	}
	if transaction.BookedOn != "2026-03-10" {
		t.Fatalf("bookedOn = %s", transaction.BookedOn)
	}
	if transaction.Description != "AMZN Mktp DE*HP1IF5XT5" {
		t.Fatalf("description = %q", transaction.Description)
	}
	if transaction.Counterparty != "AMZN.COM/BILL LU" {
		t.Fatalf("counterparty = %q", transaction.Counterparty)
	}
	if transaction.AmountCents != -38899 {
		t.Fatalf("amount = %d", transaction.AmountCents)
	}
}

func TestParseICSPDFRowHandlesCardFee(t *testing.T) {
	statementDate := time.Date(2026, time.April, 7, 0, 0, 0, 0, time.Local)
	row := pdf.TextHorizontal{
		{X: 60, S: "11 mrt."},
		{X: 103, S: "11 mrt."},
		{X: 152, S: "JAARBIJDRAGE CARD"},
		{X: 495, S: "57,95"},
		{X: 535, S: "Af"},
	}

	transaction, ok := parseICSPDFRow(row, statementDate)
	if !ok || transaction.Description != "JAARBIJDRAGE CARD" || transaction.AmountCents != -5795 {
		t.Fatalf("unexpected transaction: %+v", transaction)
	}
}
