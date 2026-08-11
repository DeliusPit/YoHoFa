package bankimport

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

const maxPDFSize = 12 << 20

var (
	icsStatementDatePattern = regexp.MustCompile(`(?i)(\d{1,2})\s+(januari|februari|maart|april|mei|juni|juli|augustus|september|oktober|november|december)\s+(\d{4})`)
	icsTransactionPattern   = regexp.MustCompile(`(?im)^\s*(\d{1,2})\s+(jan|feb|mrt|apr|mei|jun|jul|aug|sep|okt|nov|dec)\.?\s+(\d{1,2})\s+(jan|feb|mrt|apr|mei|jun|jul|aug|sep|okt|nov|dec)\.?\s+(.+?)\s+([0-9][0-9.\s]*,[0-9]{2})\s+(Af|Bij)\s*$`)
	dutchMonths             = map[string]time.Month{
		"jan": 1, "januari": 1,
		"feb": 2, "februari": 2,
		"mrt": 3, "maart": 3,
		"apr": 4, "april": 4,
		"mei": 5,
		"jun": 6, "juni": 6,
		"jul": 7, "juli": 7,
		"aug": 8, "augustus": 8,
		"sep": 9, "september": 9,
		"okt": 10, "oktober": 10,
		"nov": 11, "november": 11,
		"dec": 12, "december": 12,
	}
)

func PreviewFile(profileName string, filename string, reader io.Reader) (Preview, error) {
	if profileName != "ics" {
		return PreviewCSV(profileName, reader)
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".pdf") {
		return Preview{}, errors.New("ICS import verwacht een PDF-bestand")
	}
	return PreviewICSPDF(reader)
}

func PreviewICSPDF(reader io.Reader) (Preview, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxPDFSize+1))
	if err != nil {
		return Preview{}, err
	}
	if len(data) > maxPDFSize {
		return Preview{}, errors.New("ICS PDF is groter dan 12 MB")
	}
	if !bytes.HasPrefix(data, []byte("%PDF")) {
		return Preview{}, errors.New("ongeldig PDF-bestand")
	}

	document, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Preview{}, fmt.Errorf("PDF lezen: %w", err)
	}
	plainText, err := document.GetPlainText()
	if err != nil {
		return Preview{}, fmt.Errorf("PDF-tekst lezen: %w", err)
	}
	text, err := io.ReadAll(plainText)
	if err != nil {
		return Preview{}, fmt.Errorf("PDF-tekst lezen: %w", err)
	}
	statementDate, err := parseICSStatementDate(string(text))
	if err != nil {
		return Preview{}, err
	}
	transactions, warnings := parseICSPDFRows(document, statementDate)
	if len(transactions) > 0 {
		return Preview{Profile: "ics", Transactions: transactions, Warnings: warnings}, nil
	}
	return ParseICSText(string(text))
}

func parseICSPDFRows(document *pdf.Reader, statementDate time.Time) ([]Transaction, []string) {
	transactions := []Transaction{}
	warnings := []string{}
	for pageNumber := 1; pageNumber <= document.NumPage(); pageNumber++ {
		rows, err := document.Page(pageNumber).GetTextByRow()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("pagina %d overgeslagen: %s", pageNumber, err))
			continue
		}
		for _, row := range rows {
			transaction, ok := parseICSPDFRow(row.Content, statementDate)
			if ok {
				transactions = append(transactions, transaction)
			}
		}
	}
	return transactions, warnings
}

func parseICSPDFRow(content pdf.TextHorizontal, statementDate time.Time) (Transaction, bool) {
	if len(content) < 4 {
		return Transaction{}, false
	}

	transactionDate, err := parseICSShortDate(content[0].S, statementDate)
	if err != nil {
		return Transaction{}, false
	}
	bookingDate, err := parseICSShortDate(content[1].S, statementDate)
	if err != nil {
		return Transaction{}, false
	}

	directionIndex := -1
	for index := len(content) - 1; index >= 2; index-- {
		direction := normalize(content[index].S)
		if direction == "af" || direction == "bij" {
			directionIndex = index
			break
		}
	}
	if directionIndex < 3 {
		return Transaction{}, false
	}

	amountIndex := -1
	var amount int64
	for index := directionIndex - 1; index >= 2; index-- {
		parsed, err := parseAmount(content[index].S)
		if err == nil {
			amountIndex = index
			amount = applyDirection(parsed, content[directionIndex].S)
			break
		}
	}
	if amountIndex < 3 {
		return Transaction{}, false
	}

	descriptionParts := []string{}
	counterpartyParts := []string{}
	for _, text := range content[2:amountIndex] {
		value := strings.TrimSpace(text.S)
		if value == "" {
			continue
		}
		switch {
		case text.X < 280:
			descriptionParts = append(descriptionParts, value)
		case text.X < 400:
			counterpartyParts = append(counterpartyParts, value)
		}
	}
	description := strings.Join(descriptionParts, " ")
	if description == "" {
		return Transaction{}, false
	}
	counterparty := strings.Join(counterpartyParts, " ")
	if counterparty == "" {
		counterparty = description
	}

	return Transaction{
		BookedOn:          bookingDate.Format(time.DateOnly),
		Description:       description,
		Counterparty:      counterparty,
		AmountCents:       amount,
		ExternalReference: fmt.Sprintf("ICS-%s-%s-%s", transactionDate.Format("20060102"), bookingDate.Format("20060102"), description),
	}, true
}

func parseICSShortDate(value string, statementDate time.Time) (time.Time, error) {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid ICS date %q", value)
	}
	day, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid ICS date %q", value)
	}
	monthName := strings.TrimSuffix(strings.ToLower(parts[1]), ".")
	month, ok := dutchMonths[monthName]
	if !ok {
		return time.Time{}, fmt.Errorf("invalid ICS month %q", monthName)
	}
	year := statementDate.Year()
	if month > statementDate.Month() {
		year--
	}
	date := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
	if date.Day() != day || date.Month() != month {
		return time.Time{}, fmt.Errorf("invalid ICS date %q", value)
	}
	return date, nil
}

func ParseICSText(text string) (Preview, error) {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	statementDate, err := parseICSStatementDate(text)
	if err != nil {
		return Preview{}, err
	}

	matches := icsTransactionPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return Preview{}, errors.New("geen ICS-transacties gevonden in PDF")
	}

	preview := Preview{Profile: "ics", Transactions: []Transaction{}, Warnings: []string{}}
	for rowIndex, match := range matches {
		bookingDay, _ := strconv.Atoi(match[3])
		bookingMonth := dutchMonths[strings.ToLower(match[4])]
		bookingYear := statementDate.Year()
		if bookingMonth > statementDate.Month() {
			bookingYear--
		}
		bookingDate := time.Date(bookingYear, bookingMonth, bookingDay, 0, 0, 0, 0, time.Local)
		if bookingDate.Day() != bookingDay || bookingDate.Month() != bookingMonth {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("transactie %d overgeslagen: ongeldige boekingsdatum", rowIndex+1))
			continue
		}

		amount, err := parseAmount(match[6])
		if err != nil {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("transactie %d overgeslagen: %s", rowIndex+1, err))
			continue
		}
		amount = applyDirection(amount, match[7])
		description := strings.Join(strings.Fields(match[5]), " ")
		preview.Transactions = append(preview.Transactions, Transaction{
			BookedOn:     bookingDate.Format(time.DateOnly),
			Description:  description,
			Counterparty: description,
			AmountCents:  amount,
		})
	}
	if len(preview.Transactions) == 0 {
		return Preview{}, errors.New("geen geldige ICS-transacties gevonden in PDF")
	}
	return preview, nil
}

func parseICSStatementDate(text string) (time.Time, error) {
	match := icsStatementDatePattern.FindStringSubmatch(text)
	if len(match) == 0 {
		return time.Time{}, errors.New("afschriftdatum niet gevonden in ICS PDF")
	}
	day, _ := strconv.Atoi(match[1])
	year, _ := strconv.Atoi(match[3])
	month := dutchMonths[strings.ToLower(match[2])]
	date := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
	if date.Day() != day || date.Month() != month {
		return time.Time{}, errors.New("ongeldige afschriftdatum in ICS PDF")
	}
	return date, nil
}
