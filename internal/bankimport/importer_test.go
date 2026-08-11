package bankimport

import (
	"strings"
	"testing"
)

func TestApplyDirection(t *testing.T) {
	tests := []struct {
		direction string
		amount    int64
		want      int64
	}{
		{direction: "Af", amount: 1234, want: -1234},
		{direction: "Bij", amount: 1234, want: 1234},
		{direction: "debet", amount: 1234, want: -1234},
		{direction: "credit", amount: -1234, want: 1234},
	}

	for _, test := range tests {
		t.Run(test.direction, func(t *testing.T) {
			if got := applyDirection(test.amount, test.direction); got != test.want {
				t.Fatalf("applyDirection(%d, %q) = %d, want %d", test.amount, test.direction, got, test.want)
			}
		})
	}
}

func TestPreviewCSVParsesHeaderlessSNSHistory(t *testing.T) {
	csv := `01-01-2026,NL57SNSB0915119927,NL83ABNA0531184161,JPH HEUVING,,,,EUR,-171.19,EUR,665.00,01-01-2026,01-01-2026,8949,IOS,544419,,'NL83ABNA0531184161-JPH HEUVING-Bijdrage Hypotheek',1,'Huur & hypotheek'
14-01-2026,NL57SNSB0915119927,,,,,,EUR,493.39,EUR,-202.42,14-01-2026,14-01-2026,7913,BEA,2540695,,'Albert Heijn Online   >ZAANDAM 14.01.2026 20U58',2,'Boodschappen'
26-01-2026,NL57SNSB0915119927,NL32INGB0666261016,Zicht verzekeringsadviseurs,,,,EUR,-398.53,EUR,54.97,26-01-2026,26-01-2026,8809,OVS,740793,,'26335236-NL32INGB0666261016-Zicht verzekeringsadviseurs',3,'Overig'`

	preview, err := PreviewCSV("asn_sns_regiobank", strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Transactions) != 3 {
		t.Fatalf("got %d transactions, want 3", len(preview.Transactions))
	}
	if preview.Transactions[0].AmountCents != 66500 {
		t.Fatalf("first amount = %d, want 66500", preview.Transactions[0].AmountCents)
	}
	if preview.Transactions[1].AmountCents != -20242 {
		t.Fatalf("second amount = %d, want -20242", preview.Transactions[1].AmountCents)
	}
	if preview.Transactions[1].Counterparty != "Albert Heijn Online" {
		t.Fatalf("counterparty = %q, want Albert Heijn Online", preview.Transactions[1].Counterparty)
	}
	if preview.Transactions[2].CounterpartyIBAN != "NL32INGB0666261016" {
		t.Fatalf("counterparty IBAN = %q", preview.Transactions[2].CounterpartyIBAN)
	}
}

func TestPreviewCSVParsesHeaderlessSNSSavingsHistory(t *testing.T) {
	csv := `05-01-2026,NL92SNSB0964160072,,,,,,EUR,20.88,EUR,0.02,05-01-2026,01-01-2026,6607,BIJ,2058798,,'Rente over positief saldo tot 01-01-26',1
05-02-2026,NL92SNSB0964160072,,,,,,EUR,20.90,EUR,0.02,05-02-2026,01-02-2026,6607,BIJ,2045601,,'Rente over positief saldo tot 01-02-26',3`

	preview, err := PreviewCSV("asn_sns_regiobank", strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Transactions) != 2 {
		t.Fatalf("got %d transactions, want 2", len(preview.Transactions))
	}
	if preview.Transactions[0].AmountCents != 2 || preview.Transactions[0].Description != "Rente over positief saldo tot 01-01-26" {
		t.Fatalf("unexpected transaction: %+v", preview.Transactions[0])
	}
}
