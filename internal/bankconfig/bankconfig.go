package bankconfig

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalid = errors.New("ongeldige bankimportconfiguratie")
	slugExpr   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

type Config struct {
	Slug    string       `json:"slug"`
	Bank    Bank         `json:"bank"`
	Source  Source       `json:"source"`
	Columns Columns      `json:"columns"`
	Mapping Mapping      `json:"mapping"`
	Formats Formats      `json:"formats"`
	Amount  Amount       `json:"amount"`
	RawYAML string       `json:"rawYaml,omitempty"`
	Status  ConfigStatus `json:"status"`
}

type Bank struct {
	Name string `json:"name"`
}

type Source struct {
	Format    string `json:"format"`
	Delimiter string `json:"delimiter"`
	Encoding  string `json:"encoding"`
	Quote     string `json:"quote"`
	Header    bool   `json:"header"`
}

type Columns struct {
	TransactionDate     int `json:"transaction_date"`
	Account             int `json:"account"`
	Amount              int `json:"amount"`
	DebitCredit         int `json:"debit_credit"`
	Counterparty        int `json:"counterparty"`
	CounterpartyAccount int `json:"counterparty_account"`
	TransactionCode     int `json:"transaction_code"`
	Description         int `json:"description"`
	Balance             int `json:"balance"`
}

type Mapping struct {
	TransactionDate string `json:"transaction_date"`
	AmountCents     string `json:"amount_cents"`
	Description     string `json:"description"`
	Counterparty    string `json:"counterparty"`
}

type Formats struct {
	TransactionDate    string `json:"transaction_date"`
	DecimalSeparator   string `json:"decimal_separator"`
	ThousandsSeparator string `json:"thousands_separator"`
}

type Amount struct {
	Mode        string `json:"mode"`
	DebitValue  string `json:"debit_value"`
	CreditValue string `json:"credit_value"`
}

type ConfigStatus struct {
	Valid   bool     `json:"valid"`
	Errors  []string `json:"errors,omitempty"`
	Profile int64    `json:"profileId,omitempty"`
}

type NormalizedTransaction struct {
	TransactionDate string `json:"transaction_date"`
	AmountCents     int64  `json:"amount_cents"`
	Description     string `json:"description"`
	Counterparty    string `json:"counterparty"`
}

type ParsedRow struct {
	Row         int                   `json:"row"`
	Raw         map[string]string     `json:"raw"`
	Transaction NormalizedTransaction `json:"transaction"`
}

func List(dir string) ([]Config, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := []Config{}
	for _, entry := range entries {
		if entry.IsDir() || !(strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}
		item, err := Read(dir, strings.TrimSuffix(strings.TrimSuffix(entry.Name(), ".yaml"), ".yml"))
		if err != nil {
			item = Config{Slug: strings.TrimSuffix(strings.TrimSuffix(entry.Name(), ".yaml"), ".yml"), Status: ConfigStatus{Valid: false, Errors: []string{err.Error()}}}
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Slug < items[j].Slug })
	return items, nil
}

func Read(dir, slug string) (Config, error) {
	if !slugExpr.MatchString(slug) {
		return Config{}, fmt.Errorf("%w: ongeldige bestandsnaam", ErrInvalid)
	}
	data, err := os.ReadFile(filepath.Join(dir, slug+".yaml"))
	if err != nil {
		return Config{}, err
	}
	item, err := Parse(slug, string(data))
	item.RawYAML = string(data)
	return item, err
}

func Save(dir, slug, raw string) (Config, error) {
	if !slugExpr.MatchString(slug) {
		return Config{}, fmt.Errorf("%w: gebruik alleen kleine letters, cijfers, _ en -", ErrInvalid)
	}
	item, err := Parse(slug, raw)
	if err != nil {
		return item, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Config{}, err
	}
	path := filepath.Join(dir, slug+".yaml")
	temporary, err := os.CreateTemp(dir, ".bank-*.yaml")
	if err != nil {
		return Config{}, err
	}
	name := temporary.Name()
	if _, err := temporary.WriteString(strings.TrimSpace(raw) + "\n"); err != nil {
		temporary.Close()
		os.Remove(name)
		return Config{}, err
	}
	if err := temporary.Close(); err != nil {
		os.Remove(name)
		return Config{}, err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		os.Remove(name)
		return Config{}, err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return Config{}, err
	}
	item.RawYAML = strings.TrimSpace(raw) + "\n"
	return item, nil
}

func Delete(dir, slug string) error {
	if !slugExpr.MatchString(slug) {
		return fmt.Errorf("%w: ongeldige bestandsnaam", ErrInvalid)
	}
	err := os.Remove(filepath.Join(dir, slug+".yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func Parse(slug, raw string) (Config, error) {
	item := Config{Slug: slug, RawYAML: raw}
	item.Source.Header = true
	item.Source.Quote = "\""
	item.Columns = defaultColumns()
	section := ""
	for number, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && strings.HasSuffix(trimmed, ":") {
			section = strings.TrimSuffix(trimmed, ":")
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 || section == "" {
			return item, fmt.Errorf("%w: regel %d kan niet worden gelezen", ErrInvalid, number+1)
		}
		key := strings.TrimSpace(parts[0])
		value := unquote(strings.TrimSpace(parts[1]))
		switch section + "." + key {
		case "bank.name":
			item.Bank.Name = value
		case "source.format":
			item.Source.Format = value
		case "source.delimiter":
			item.Source.Delimiter = value
		case "source.encoding":
			item.Source.Encoding = value
		case "source.quote":
			item.Source.Quote = value
		case "source.header":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return item, fmt.Errorf("%w: source.header moet true of false zijn", ErrInvalid)
			}
			item.Source.Header = parsed
		case "columns.transaction_date":
			item.Columns.TransactionDate = parseColumn(value)
		case "columns.account":
			item.Columns.Account = parseColumn(value)
		case "columns.amount":
			item.Columns.Amount = parseColumn(value)
		case "columns.debit_credit":
			item.Columns.DebitCredit = parseColumn(value)
		case "columns.counterparty":
			item.Columns.Counterparty = parseColumn(value)
		case "columns.counterparty_account":
			item.Columns.CounterpartyAccount = parseColumn(value)
		case "columns.transaction_code":
			item.Columns.TransactionCode = parseColumn(value)
		case "columns.description":
			item.Columns.Description = parseColumn(value)
		case "columns.balance":
			item.Columns.Balance = parseColumn(value)
		case "mapping.transaction_date":
			item.Mapping.TransactionDate = value
		case "mapping.amount_cents":
			item.Mapping.AmountCents = value
		case "mapping.description":
			item.Mapping.Description = value
		case "mapping.counterparty":
			item.Mapping.Counterparty = value
		case "formats.transaction_date":
			item.Formats.TransactionDate = value
		case "formats.decimal_separator":
			item.Formats.DecimalSeparator = value
		case "formats.thousands_separator":
			item.Formats.ThousandsSeparator = value
		case "amount.mode":
			item.Amount.Mode = value
		case "amount.debit_value":
			item.Amount.DebitValue = value
		case "amount.credit_value":
			item.Amount.CreditValue = value
		default:
			return item, fmt.Errorf("%w: onbekend veld %s.%s", ErrInvalid, section, key)
		}
	}
	item.Status.Errors = validate(item)
	item.Status.Valid = len(item.Status.Errors) == 0
	if !item.Status.Valid {
		return item, fmt.Errorf("%w: %s", ErrInvalid, strings.Join(item.Status.Errors, "; "))
	}
	return item, nil
}

func defaultColumns() Columns {
	return Columns{TransactionDate: -1, Account: -1, Amount: -1, DebitCredit: -1, Counterparty: -1, CounterpartyAccount: -1, TransactionCode: -1, Description: -1, Balance: -1}
}

func validate(item Config) []string {
	var errs []string
	if strings.TrimSpace(item.Bank.Name) == "" {
		errs = append(errs, "bank.name is verplicht")
	}
	if item.Source.Format != "csv" {
		errs = append(errs, "source.format moet csv zijn")
	}
	if item.Source.Delimiter == "" {
		errs = append(errs, "source.delimiter is verplicht")
	}
	if item.Source.Quote != "\"" {
		errs = append(errs, "source.quote moet \" zijn")
	}
	if item.Source.Encoding != "utf-8" {
		errs = append(errs, "source.encoding moet utf-8 zijn")
	}
	if !item.Source.Header && (item.Columns.TransactionDate < 0 || item.Columns.Amount < 0 || item.Columns.Description < 0 || item.Columns.Counterparty < 0) {
		errs = append(errs, "columns moet indexen bevatten voor transaction_date, amount, description en counterparty")
	}
	if item.Mapping.TransactionDate == "" || item.Mapping.AmountCents == "" || item.Mapping.Description == "" || item.Mapping.Counterparty == "" {
		errs = append(errs, "alle mappingvelden zijn verplicht")
	}
	if item.Formats.TransactionDate == "" {
		errs = append(errs, "formats.transaction_date is verplicht")
	}
	if item.Formats.DecimalSeparator != "," && item.Formats.DecimalSeparator != "." {
		errs = append(errs, "formats.decimal_separator moet , of . zijn")
	}
	if item.Amount.Mode != "" && item.Amount.Mode != "debit_credit" {
		errs = append(errs, "amount.mode moet debit_credit zijn")
	}
	if item.Amount.Mode == "debit_credit" && (item.Amount.DebitValue == "" || item.Amount.CreditValue == "") {
		errs = append(errs, "amount.debit_value en amount.credit_value zijn verplicht")
	}
	return errs
}

func Normalize(item Config, raw map[string]string) (NormalizedTransaction, error) {
	if errs := validate(item); len(errs) != 0 {
		return NormalizedTransaction{}, fmt.Errorf("%w: %s", ErrInvalid, strings.Join(errs, "; "))
	}
	date, err := parseDate(raw[item.Mapping.TransactionDate], item.Formats.TransactionDate)
	if err != nil {
		return NormalizedTransaction{}, err
	}
	amount, err := parseAmount(raw[item.Mapping.AmountCents], item.Formats.DecimalSeparator, item.Formats.ThousandsSeparator)
	if err != nil {
		return NormalizedTransaction{}, err
	}
	if item.Amount.Mode == "debit_credit" {
		direction := strings.TrimSpace(raw["debit_credit"])
		if direction == item.Amount.DebitValue && amount > 0 {
			amount = -amount
		} else if direction == item.Amount.CreditValue && amount < 0 {
			amount = -amount
		} else if direction != item.Amount.DebitValue && direction != item.Amount.CreditValue {
			return NormalizedTransaction{}, fmt.Errorf("%w: onbekende debet/credit waarde", ErrInvalid)
		}
	}
	return NormalizedTransaction{TransactionDate: date, AmountCents: amount, Description: strings.TrimSpace(raw[item.Mapping.Description]), Counterparty: strings.TrimSpace(raw[item.Mapping.Counterparty])}, nil
}

func PreviewCSV(item Config, reader io.Reader, limit int) ([]NormalizedTransaction, error) {
	rows, err := ParseCSVRows(item, reader, limit)
	if err != nil {
		return nil, err
	}
	items := make([]NormalizedTransaction, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.Transaction)
	}
	return items, nil
}

func ParseCSVRows(item Config, reader io.Reader, limit int) ([]ParsedRow, error) {
	if item.Source.Format != "csv" {
		return nil, fmt.Errorf("%w: alleen csv wordt ondersteund", ErrInvalid)
	}
	r := csv.NewReader(reader)
	delimiter := []rune(item.Source.Delimiter)
	if len(delimiter) != 1 {
		return nil, fmt.Errorf("%w: delimiter moet één teken zijn", ErrInvalid)
	}
	r.Comma = delimiter[0]
	r.FieldsPerRecord = -1
	var header []string
	if item.Source.Header {
		var err error
		header, err = r.Read()
		if err != nil {
			return nil, err
		}
	} else {
		header = item.columnHeaders()
	}
	items := []ParsedRow{}
	rowNumber := 0
	if item.Source.Header {
		rowNumber = 1
	}
	for limit <= 0 || len(items) < limit {
		record, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		rowNumber++
		raw := rawFromRecord(header, record)
		transaction, err := Normalize(item, raw)
		if err != nil {
			return nil, fmt.Errorf("rij %d: %w", rowNumber, err)
		}
		items = append(items, ParsedRow{Row: rowNumber, Raw: raw, Transaction: transaction})
	}
	return items, nil
}

func ConfigurationJSON(item Config) (string, error) {
	data, err := json.Marshal(item)
	return string(data), err
}

func Template(slug string) string {
	name := strings.Title(strings.ReplaceAll(strings.ReplaceAll(slug, "-", " "), "_", " "))
	return "bank:\n  name: \"" + name + "\"\n\nsource:\n  format: csv\n  delimiter: \";\"\n  encoding: utf-8\n\nmapping:\n  transaction_date: Buchungstag\n  amount_cents: Betrag\n  description: Verwendungszweck\n  counterparty: Auftraggever/Ontvanger\n\nformats:\n  transaction_date: \"02.01.2006\"\n  decimal_separator: \",\"\n"
}

func (item Config) columnHeaders() []string {
	columns := map[int]string{
		item.Columns.TransactionDate:     "transaction_date",
		item.Columns.Account:             "account",
		item.Columns.Amount:              "amount",
		item.Columns.DebitCredit:         "debit_credit",
		item.Columns.Counterparty:        "counterparty",
		item.Columns.CounterpartyAccount: "counterparty_account",
		item.Columns.TransactionCode:     "transaction_code",
		item.Columns.Description:         "description",
		item.Columns.Balance:             "balance",
	}
	maxColumn := 0
	for index := range columns {
		if index > maxColumn {
			maxColumn = index
		}
	}
	headers := make([]string, maxColumn+1)
	for index, name := range columns {
		if index >= 0 {
			headers[index] = name
		}
	}
	return headers
}

func rawFromRecord(header, record []string) map[string]string {
	raw := map[string]string{}
	for i, name := range header {
		if i < len(record) && name != "" {
			raw[name] = record[i]
		}
	}
	return raw
}

func unquote(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		return value[1 : len(value)-1]
	}
	return value
}

func parseDate(value, layout string) (string, error) {
	date, err := time.Parse(layout, strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("%w: datum kan niet worden gelezen", ErrInvalid)
	}
	return date.Format("2006-01-02"), nil
}

func parseColumn(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return -1
	}
	return parsed
}

func parseAmount(value, decimal string, thousands string) (int64, error) {
	cleaned := strings.TrimSpace(value)
	if thousands != "" {
		cleaned = strings.ReplaceAll(cleaned, thousands, "")
	} else if decimal == "," {
		cleaned = strings.ReplaceAll(cleaned, ".", "")
	} else if decimal == "." {
		cleaned = strings.ReplaceAll(cleaned, ",", "")
	}
	if decimal != "." {
		cleaned = strings.Replace(cleaned, decimal, ".", 1)
	}
	negative := strings.HasPrefix(cleaned, "-")
	cleaned = strings.TrimPrefix(strings.TrimPrefix(cleaned, "+"), "-")
	parts := strings.SplitN(cleaned, ".", 3)
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("%w: bedrag kan niet worden gelezen", ErrInvalid)
	}
	cents := "00"
	if len(parts) == 2 {
		cents = (parts[1] + "00")[:2]
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: bedrag kan niet worden gelezen", ErrInvalid)
	}
	fraction, err := strconv.ParseInt(cents, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: bedrag kan niet worden gelezen", ErrInvalid)
	}
	amount := whole*100 + fraction
	if negative {
		amount = -amount
	}
	return amount, nil
}
