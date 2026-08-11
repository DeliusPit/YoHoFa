package bankimport

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const noColumn = -1

type Transaction struct {
	BookedOn          string `json:"bookedOn"`
	Description       string `json:"description"`
	Counterparty      string `json:"counterparty,omitempty"`
	CounterpartyIBAN  string `json:"counterpartyIban,omitempty"`
	AmountCents       int64  `json:"amountCents"`
	ExternalReference string `json:"externalReference,omitempty"`
}

type Preview struct {
	Profile      string        `json:"profile"`
	Transactions []Transaction `json:"transactions"`
	Warnings     []string      `json:"warnings"`
}

type Profile struct {
	Name            string
	Headerless      bool
	DateColumn      int
	AmountColumn    int
	DirectionColumn int
	DescriptionCol  int
	CounterpartyCol int
	IBANColumn      int
	ReferenceCol    int
	DateHeaders     []string
	AmountHeaders   []string
	DebitHeaders    []string
	CreditHeaders   []string
	DescriptionKeys []string
	Counterparty    []string
	IBANHeaders     []string
	Reference       []string
}

var profiles = map[string]Profile{
	"generic": {
		Name:            "generic",
		DateHeaders:     []string{"date", "datum", "booked_on", "boekdatum", "booking date"},
		AmountHeaders:   []string{"amount", "bedrag", "bedrag eur", "amount eur"},
		DebitHeaders:    []string{"af", "debit", "debet"},
		CreditHeaders:   []string{"bij", "credit", "creditering"},
		DescriptionKeys: []string{"description", "omschrijving", "mededelingen", "details"},
		Counterparty:    []string{"counterparty", "tegenpartij", "naam", "naam omschrijving"},
		IBANHeaders:     []string{"iban", "tegenrekening", "rekening tegenpartij"},
		Reference:       []string{"reference", "referentie", "transactie id", "transaction id"},
	},
	"ing": {
		Name:            "ing",
		DateHeaders:     []string{"datum", "date"},
		AmountHeaders:   []string{"bedrag eur", "bedrag"},
		DescriptionKeys: []string{"naam omschrijving", "omschrijving", "mededelingen"},
		Counterparty:    []string{"naam tegenpartij", "naam omschrijving"},
		IBANHeaders:     []string{"tegenrekening", "iban"},
		Reference:       []string{"code", "af bij"},
	},
	"rabobank": {
		Name:            "rabobank",
		DateHeaders:     []string{"datum", "rentedatum", "boekdatum"},
		AmountHeaders:   []string{"bedrag", "amount"},
		DescriptionKeys: []string{"omschrijving", "mededelingen"},
		Counterparty:    []string{"naam tegenpartij", "tegenpartij"},
		IBANHeaders:     []string{"tegenrekening iban/nummer", "tegenrekening", "iban"},
		Reference:       []string{"transactiereferentie", "referentie"},
	},
	"abnamro": {
		Name:            "abnamro",
		DateHeaders:     []string{"datum", "transaction date", "book date"},
		AmountHeaders:   []string{"bedrag", "amount"},
		DescriptionKeys: []string{"omschrijving", "description"},
		Counterparty:    []string{"naam tegenpartij", "counterparty"},
		IBANHeaders:     []string{"tegenrekening", "counterparty account"},
		Reference:       []string{"mutatiecode", "reference"},
	},
	"asn_sns_regiobank": {
		Name:            "asn_sns_regiobank",
		DateHeaders:     []string{"boekdatum", "datum"},
		AmountHeaders:   []string{"bedrag", "amount"},
		DescriptionKeys: []string{"omschrijving", "mededelingen"},
		Counterparty:    []string{"naam tegenpartij", "tegenpartij"},
		IBANHeaders:     []string{"tegenrekening", "iban tegenpartij"},
		Reference:       []string{"transactie id", "referentie"},
	},
	"sparda": {
		Name:            "sparda",
		DateHeaders:     []string{"buchungstag", "buchung", "valuta", "wertstellung"},
		AmountHeaders:   []string{"umsatz", "betrag", "amount"},
		DescriptionKeys: []string{"verwendungszweck", "buchungstext", "beschreibung"},
		Counterparty:    []string{"beguenstigter zahlungspflichtiger", "beg\u00fcnstigter zahlungspflichtiger", "auftraggeber empfaenger", "auftraggeber empf\u00e4nger", "name"},
		IBANHeaders:     []string{"iban", "iban zahlungspflichtiger", "iban auftraggeber"},
		Reference:       []string{"mandatsreferenz", "kundenreferenz", "referenz"},
	},
	"triodos": {
		Name:            "triodos",
		Headerless:      true,
		DateColumn:      0,
		AmountColumn:    2,
		DirectionColumn: 3,
		CounterpartyCol: 4,
		IBANColumn:      5,
		ReferenceCol:    6,
		DescriptionCol:  7,
	},
}

func ProfileNames() []string {
	return []string{"generic", "ing", "rabobank", "abnamro", "asn_sns_regiobank", "sparda", "triodos", "ics"}
}

func PreviewCSV(profileName string, reader io.Reader) (Preview, error) {
	profile, ok := profiles[profileName]
	if !ok {
		return Preview{}, fmt.Errorf("unknown bank profile %q", profileName)
	}

	data, err := io.ReadAll(io.LimitReader(reader, 5<<20))
	if err != nil {
		return Preview{}, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return Preview{}, errors.New("empty import file")
	}

	csvReader := csv.NewReader(strings.NewReader(string(data)))
	csvReader.Comma = detectDelimiter(string(data))
	csvReader.FieldsPerRecord = -1
	csvReader.TrimLeadingSpace = true

	records, err := csvReader.ReadAll()
	if err != nil {
		return Preview{}, err
	}
	if profileName == "asn_sns_regiobank" && isSNSHistoryExport(records) {
		preview := parseSNSHistoryExport(records)
		if len(preview.Transactions) == 0 {
			return Preview{}, errors.New("no transactions found in SNS transaction history")
		}
		return preview, nil
	}
	if profile.Headerless && len(records) < 1 {
		return Preview{}, errors.New("import file needs at least one transaction")
	}
	if !profile.Headerless && len(records) < 2 {
		return Preview{}, errors.New("import file needs a header row and at least one transaction")
	}

	preview := Preview{Profile: profile.Name, Transactions: []Transaction{}, Warnings: []string{}}
	if profile.Headerless {
		for rowIndex, record := range records {
			transaction, err := parseHeaderlessRow(profile, record)
			if err != nil {
				preview.Warnings = append(preview.Warnings, fmt.Sprintf("row %d skipped: %s", rowIndex+1, err))
				continue
			}
			preview.Transactions = append(preview.Transactions, transaction)
		}
		return preview, nil
	}

	headers := headerIndex(records[0])
	for rowIndex, record := range records[1:] {
		transaction, err := parseRow(profile, headers, record)
		if err != nil {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("row %d skipped: %s", rowIndex+2, err))
			continue
		}
		preview.Transactions = append(preview.Transactions, transaction)
	}

	return preview, nil
}

func isSNSHistoryExport(records [][]string) bool {
	if len(records) == 0 || len(records[0]) < 19 {
		return false
	}
	record := records[0]
	if _, err := parseDate(column(record, 0)); err != nil {
		return false
	}
	if !strings.HasPrefix(strings.ToUpper(column(record, 1)), "NL") {
		return false
	}
	if !strings.EqualFold(column(record, 7), "EUR") || !strings.EqualFold(column(record, 9), "EUR") {
		return false
	}
	_, err := parseAmount(column(record, 10))
	return err == nil
}

func parseSNSHistoryExport(records [][]string) Preview {
	preview := Preview{Profile: "asn_sns_regiobank", Transactions: []Transaction{}, Warnings: []string{}}
	for rowIndex, record := range records {
		if len(record) < 19 {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("row %d skipped: expected at least 19 columns", rowIndex+1))
			continue
		}
		bookedOn, err := parseDate(column(record, 0))
		if err != nil {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("row %d skipped: %s", rowIndex+1, err))
			continue
		}
		amount, err := parseAmount(column(record, 10))
		if err != nil {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("row %d skipped: %s", rowIndex+1, err))
			continue
		}
		description := strings.Trim(strings.TrimSpace(column(record, 17)), "'")
		if description == "" {
			description = strings.Trim(strings.TrimSpace(column(record, 3)), "'")
		}
		if description == "" {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("row %d skipped: missing description", rowIndex+1))
			continue
		}
		counterparty := strings.Trim(strings.TrimSpace(column(record, 3)), "'")
		if counterparty == "" {
			counterparty = snsCounterparty(description)
		}
		preview.Transactions = append(preview.Transactions, Transaction{
			BookedOn:          bookedOn.Format(time.DateOnly),
			Description:       description,
			Counterparty:      counterparty,
			CounterpartyIBAN:  extractIBAN(column(record, 2)),
			AmountCents:       amount,
			ExternalReference: strings.Join([]string{column(record, 13), column(record, 14), column(record, 15)}, "-"),
		})
	}
	return preview
}

func snsCounterparty(description string) string {
	if index := strings.Index(description, ">"); index > 0 {
		return strings.TrimSpace(description[:index])
	}
	if index := strings.Index(description, ":"); index > 0 {
		return strings.TrimSpace(description[:index])
	}
	return description
}

func parseHeaderlessRow(profile Profile, record []string) (Transaction, error) {
	bookedOn, err := parseDate(column(record, profile.DateColumn))
	if err != nil {
		return Transaction{}, err
	}

	amount, err := parseAmount(column(record, profile.AmountColumn))
	if err != nil {
		return Transaction{}, err
	}
	amount = applyDirection(amount, column(record, profile.DirectionColumn))

	description := column(record, profile.DescriptionCol)
	if description == "" {
		description = column(record, profile.CounterpartyCol)
	}
	if description == "" {
		return Transaction{}, errors.New("missing description")
	}

	return Transaction{
		BookedOn:          bookedOn.Format(time.DateOnly),
		Description:       description,
		Counterparty:      column(record, profile.CounterpartyCol),
		CounterpartyIBAN:  extractIBAN(column(record, profile.IBANColumn)),
		AmountCents:       amount,
		ExternalReference: column(record, profile.ReferenceCol),
	}, nil
}

func parseRow(profile Profile, headers map[string]int, record []string) (Transaction, error) {
	bookedOn, err := parseDate(value(record, headers, profile.DateHeaders))
	if err != nil {
		return Transaction{}, err
	}

	amount, err := parseAmount(value(record, headers, profile.AmountHeaders))
	if err != nil {
		debit := value(record, headers, profile.DebitHeaders)
		credit := value(record, headers, profile.CreditHeaders)
		amount, err = parseDebitCredit(debit, credit)
		if err != nil {
			return Transaction{}, err
		}
	}

	description := value(record, headers, profile.DescriptionKeys)
	if description == "" {
		return Transaction{}, errors.New("missing description")
	}

	return Transaction{
		BookedOn:          bookedOn.Format(time.DateOnly),
		Description:       description,
		Counterparty:      value(record, headers, profile.Counterparty),
		CounterpartyIBAN:  value(record, headers, profile.IBANHeaders),
		AmountCents:       amount,
		ExternalReference: value(record, headers, profile.Reference),
	}, nil
}

func detectDelimiter(data string) rune {
	firstLine := data
	if index := strings.IndexByte(data, '\n'); index >= 0 {
		firstLine = data[:index]
	}
	if strings.Count(firstLine, ";") >= strings.Count(firstLine, ",") {
		return ';'
	}
	return ','
}

func headerIndex(headers []string) map[string]int {
	index := make(map[string]int, len(headers))
	for i, header := range headers {
		index[normalize(header)] = i
	}
	return index
}

func value(record []string, headers map[string]int, candidates []string) string {
	for _, candidate := range candidates {
		index, ok := headers[normalize(candidate)]
		if ok && index < len(record) {
			return strings.TrimSpace(record[index])
		}
	}
	return ""
}

func column(record []string, index int) string {
	if index == noColumn || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

func normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("/", " ", "-", " ", "_", " ", ".", " ", "  ", " ").Replace(value)
	value = strings.ReplaceAll(value, "\u00e4", "ae")
	value = strings.ReplaceAll(value, "\u00f6", "oe")
	value = strings.ReplaceAll(value, "\u00fc", "ue")
	return strings.Join(strings.Fields(value), " ")
}

func parseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02", "02-01-2006", "02/01/2006", "02.01.2006", "20060102"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q", value)
}

func parseDebitCredit(debit string, credit string) (int64, error) {
	if strings.TrimSpace(debit) != "" {
		amount, err := parseAmount(debit)
		return -abs(amount), err
	}
	if strings.TrimSpace(credit) != "" {
		amount, err := parseAmount(credit)
		return abs(amount), err
	}
	return 0, errors.New("missing amount")
}

func parseAmount(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("missing amount")
	}

	value = strings.ReplaceAll(value, "EUR", "")
	value = strings.ReplaceAll(value, "\u20ac", "")
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "'", "")

	if strings.Contains(value, ",") {
		value = strings.ReplaceAll(value, ".", "")
		value = strings.ReplaceAll(value, ",", ".")
	}

	amount, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q", value)
	}
	return int64(math.Round(amount * 100)), nil
}

func applyDirection(amount int64, direction string) int64 {
	switch normalize(direction) {
	case "af", "debet", "debit", "debet af", "debet debit":
		return -abs(amount)
	case "bij", "credit", "credit bij":
		return abs(amount)
	default:
		return amount
	}
}

func extractIBAN(value string) string {
	ibanPattern := regexp.MustCompile(`[A-Z]{2}[0-9A-Z]{13,32}`)
	return ibanPattern.FindString(strings.ReplaceAll(strings.ToUpper(value), " ", ""))
}

func abs(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
