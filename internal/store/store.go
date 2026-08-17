package store

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type Backend interface {
	Close()
	Ping(ctx context.Context) error
	Summary(ctx context.Context) (Summary, error)
	MainCategories(ctx context.Context) ([]MainCategory, error)
	CreateMainCategory(ctx context.Context, input CreateMainCategory) (MainCategory, error)
	UpdateMainCategory(ctx context.Context, id int64, input UpdateMainCategory) (MainCategory, error)
	DeleteMainCategory(ctx context.Context, id int64) error
	Categories(ctx context.Context) ([]Category, error)
	CreateCategory(ctx context.Context, input CreateCategory) (Category, error)
	UpdateCategory(ctx context.Context, id int64, input UpdateCategory) (Category, error)
	DeleteCategory(ctx context.Context, id int64) error
	Accounts(ctx context.Context) ([]Account, error)
	CreateAccount(ctx context.Context, input SaveAccount) (Account, error)
	UpdateAccount(ctx context.Context, id int64, input SaveAccount) (Account, error)
	DeleteAccount(ctx context.Context, id int64) error
	ImportTransactions(ctx context.Context, accountID int64, transactions []ImportedTransaction) (ImportResult, error)
	Transactions(ctx context.Context, filter TransactionFilter) ([]Transaction, error)
	TransactionYears(ctx context.Context) ([]int, error)
	TransactionSummary(ctx context.Context, filter TransactionFilter) (TransactionSummary, error)
	CategorizeTransaction(ctx context.Context, id int64, input CategorizeTransaction) (CategorizeResult, error)
	CategoryMonthlyTotals(ctx context.Context, filter CategoryReportFilter) ([]MonthlyTotal, error)
	MainCategoryPeriodTotals(ctx context.Context, filter PeriodReportFilter) ([]CategoryTotal, error)
	SetTransactionHidden(ctx context.Context, id int64, hidden bool) error
}

type Summary struct {
	Households   int `json:"households"`
	Accounts     int `json:"accounts"`
	Transactions int `json:"transactions"`
}

type Category struct {
	ID        int64  `json:"id"`
	MainID    int64  `json:"mainCategoryId,omitempty"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Purpose   string `json:"purpose"`
	SortOrder int    `json:"sortOrder"`
}

type Account struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	IBAN         string `json:"iban"`
	Type         string `json:"type"`
	Owner        string `json:"owner"`
	BankProfile  string `json:"bankProfile"`
	SortOrder    int    `json:"sortOrder"`
	BalanceCents int64  `json:"balanceCents"`
}

type Transaction struct {
	ID                int64  `json:"id"`
	AccountID         int64  `json:"accountId"`
	CategoryID        int64  `json:"categoryId,omitempty"`
	BookedOn          string `json:"bookedOn"`
	Description       string `json:"description"`
	Counterparty      string `json:"counterparty,omitempty"`
	CounterpartyIBAN  string `json:"counterpartyIban,omitempty"`
	AmountCents       int64  `json:"amountCents"`
	ExternalReference string `json:"externalReference,omitempty"`
	ImportFingerprint string `json:"importFingerprint,omitempty"`
	Hidden            bool   `json:"hidden"`
	DocumentCount     int    `json:"documentCount"`
}

type TransactionFilter struct {
	AccountID      int64
	MainCategoryID int64
	CategoryID     int64
	From           string
	To             string
	Search         string
	Uncategorized  bool
	HiddenOnly     bool
	Limit          int
	Offset         int
}

type TransactionSummary struct {
	Count       int   `json:"count"`
	AmountCents int64 `json:"amountCents"`
}

type CategorizeTransaction struct {
	CategoryID              int64 `json:"categoryId"`
	ApplyToSameCounterparty bool  `json:"applyToSameCounterparty"`
}

type CategorizeResult struct {
	Updated int `json:"updated"`
}

type CategoryReportFilter struct {
	MainCategoryID int64
	CategoryID     int64
}

type PeriodReportFilter struct {
	MainCategoryID int64
	From           string
	To             string
}

type MonthlyTotal struct {
	Month       string `json:"month"`
	AmountCents int64  `json:"amountCents"`
}

type CategoryTotal struct {
	MainCategoryID   int64  `json:"mainCategoryId"`
	MainCategoryName string `json:"mainCategoryName"`
	CategoryID       int64  `json:"categoryId,omitempty"`
	CategoryName     string `json:"categoryName,omitempty"`
	AmountCents      int64  `json:"amountCents"`
}

type ImportedTransaction struct {
	BookedOn          string
	Description       string
	Counterparty      string
	CounterpartyIBAN  string
	AmountCents       int64
	ExternalReference string
	ImportFingerprint string
}

type ImportResult struct {
	Imported   int `json:"imported"`
	Duplicates int `json:"duplicates"`
	Total      int `json:"total"`
}

type MainCategory struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
}

type CreateMainCategory struct {
	Name string `json:"name"`
}

type UpdateMainCategory struct {
	Name string `json:"name"`
}

type CreateCategory struct {
	MainID  int64  `json:"mainCategoryId"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Purpose string `json:"purpose"`
}

type UpdateCategory struct {
	MainID  int64  `json:"mainCategoryId"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Purpose string `json:"purpose"`
}

type SaveAccount struct {
	Name        string `json:"name"`
	IBAN        string `json:"iban"`
	Type        string `json:"type"`
	Owner       string `json:"owner"`
	BankProfile string `json:"bankProfile"`
}

type Memory struct {
	mu             sync.Mutex
	summary        Summary
	accounts       []Account
	mainCategories []MainCategory
	categories     []Category
}

func Open(ctx context.Context, backend string, databaseURL string, databasePath string) (Backend, error) {
	switch backend {
	case "memory":
		return OpenMemory(), nil
	case "sqlite":
		return OpenSQLite(ctx, databasePath)
	case "postgres":
		return OpenPostgres(ctx, databaseURL)
	default:
		return nil, fmt.Errorf("unknown store backend %q", backend)
	}
}

func OpenMemory() *Memory {
	return &Memory{
		accounts: []Account{
			{ID: 1, Name: "Triodos betaalrekening", IBAN: "NL18TRIO0254363105", Type: "checking", Owner: "Huishouden", BankProfile: "triodos", SortOrder: 10},
		},
		mainCategories: []MainCategory{
			{ID: 1, Name: "Inkomsten", SortOrder: 10},
			{ID: 2, Name: "Wonen", SortOrder: 20},
			{ID: 3, Name: "Huishouden", SortOrder: 30},
			{ID: 4, Name: "Medische kosten", SortOrder: 40},
			{ID: 5, Name: "Verzekeringen", SortOrder: 50},
			{ID: 6, Name: "Telecom", SortOrder: 60},
			{ID: 7, Name: "Vervoer", SortOrder: 70},
			{ID: 8, Name: "Educatie", SortOrder: 80},
			{ID: 9, Name: "Kleding", SortOrder: 90},
			{ID: 10, Name: "Vrije tijd", SortOrder: 100},
			{ID: 11, Name: "Overige uitgaven", SortOrder: 110},
		},
		categories: []Category{
			{ID: 1, MainID: 3, Name: "Boodschappen", Kind: "expense", Purpose: "daily", SortOrder: 10},
			{ID: 2, MainID: 2, Name: "Wonen", Kind: "expense", Purpose: "daily", SortOrder: 20},
			{ID: 3, MainID: 10, Name: "Vakantie", Kind: "expense", Purpose: "reservation", SortOrder: 30},
			{ID: 4, MainID: 2, Name: "Onderhoud huis", Kind: "expense", Purpose: "reservation", SortOrder: 40},
			{ID: 5, MainID: 2, Name: "Verduurzaming woning", Kind: "expense", Purpose: "investment", SortOrder: 50},
			{ID: 6, MainID: 2, Name: "Verbouwing", Kind: "expense", Purpose: "investment", SortOrder: 60},
			{ID: 7, MainID: 1, Name: "Salaris", Kind: "income", Purpose: "daily", SortOrder: 70},
		},
	}
}

func (s *Memory) Close() {}

func (s *Memory) Ping(ctx context.Context) error {
	return ctx.Err()
}

func (s *Memory) Summary(ctx context.Context) (Summary, error) {
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}
	return s.summary, nil
}

func (s *Memory) MainCategories(ctx context.Context) ([]MainCategory, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]MainCategory(nil), s.mainCategories...), nil
}

func (s *Memory) CreateMainCategory(ctx context.Context, input CreateMainCategory) (MainCategory, error) {
	if err := ctx.Err(); err != nil {
		return MainCategory{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return MainCategory{}, fmt.Errorf("name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var nextID int64 = 1
	var sortOrder = 10
	for _, mainCategory := range s.mainCategories {
		if strings.EqualFold(mainCategory.Name, name) {
			return MainCategory{}, fmt.Errorf("main category already exists")
		}
		if mainCategory.ID >= nextID {
			nextID = mainCategory.ID + 1
		}
		if mainCategory.SortOrder >= sortOrder {
			sortOrder = mainCategory.SortOrder + 10
		}
	}

	mainCategory := MainCategory{ID: nextID, Name: name, SortOrder: sortOrder}
	s.mainCategories = append(s.mainCategories, mainCategory)
	return mainCategory, nil
}

func (s *Memory) UpdateMainCategory(ctx context.Context, id int64, input UpdateMainCategory) (MainCategory, error) {
	if err := ctx.Err(); err != nil {
		return MainCategory{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return MainCategory{}, fmt.Errorf("name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, mainCategory := range s.mainCategories {
		if mainCategory.ID != id && strings.EqualFold(mainCategory.Name, name) {
			return MainCategory{}, fmt.Errorf("main category already exists")
		}
	}
	for index, mainCategory := range s.mainCategories {
		if mainCategory.ID == id {
			s.mainCategories[index].Name = name
			return s.mainCategories[index], nil
		}
	}
	return MainCategory{}, fmt.Errorf("main category not found")
}

func (s *Memory) DeleteMainCategory(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	mainCategories := s.mainCategories[:0]
	for _, mainCategory := range s.mainCategories {
		if mainCategory.ID == id {
			found = true
			continue
		}
		mainCategories = append(mainCategories, mainCategory)
	}
	if !found {
		return fmt.Errorf("main category not found")
	}
	s.mainCategories = mainCategories

	categories := s.categories[:0]
	for _, category := range s.categories {
		if category.MainID != id {
			categories = append(categories, category)
		}
	}
	s.categories = categories
	return nil
}

func (s *Memory) Categories(ctx context.Context) ([]Category, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Category(nil), s.categories...), nil
}

func (s *Memory) Accounts(ctx context.Context) ([]Account, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Account(nil), s.accounts...), nil
}

func (s *Memory) CreateAccount(ctx context.Context, input SaveAccount) (Account, error) {
	if err := ctx.Err(); err != nil {
		return Account{}, err
	}
	account, err := normalizeAccountInput(input)
	if err != nil {
		return Account{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	var nextID int64 = 1
	account.SortOrder = 10
	for _, existing := range s.accounts {
		if account.IBAN != "" && strings.EqualFold(existing.IBAN, account.IBAN) {
			return Account{}, fmt.Errorf("account already exists")
		}
		if existing.ID >= nextID {
			nextID = existing.ID + 1
		}
		if existing.SortOrder >= account.SortOrder {
			account.SortOrder = existing.SortOrder + 10
		}
	}
	account.ID = nextID
	s.accounts = append(s.accounts, account)
	s.summary.Accounts = len(s.accounts)
	return account, nil
}

func (s *Memory) UpdateAccount(ctx context.Context, id int64, input SaveAccount) (Account, error) {
	if err := ctx.Err(); err != nil {
		return Account{}, err
	}
	account, err := normalizeAccountInput(input)
	if err != nil {
		return Account{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.accounts {
		if existing.ID != id && account.IBAN != "" && strings.EqualFold(existing.IBAN, account.IBAN) {
			return Account{}, fmt.Errorf("account already exists")
		}
	}
	for index, existing := range s.accounts {
		if existing.ID == id {
			account.ID = existing.ID
			account.SortOrder = existing.SortOrder
			s.accounts[index] = account
			return account, nil
		}
	}
	return Account{}, fmt.Errorf("account not found")
}

func (s *Memory) DeleteAccount(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	accounts := s.accounts[:0]
	for _, account := range s.accounts {
		if account.ID == id {
			found = true
			continue
		}
		accounts = append(accounts, account)
	}
	if !found {
		return fmt.Errorf("account not found")
	}
	s.accounts = accounts
	s.summary.Accounts = len(s.accounts)
	return nil
}

func (s *Memory) ImportTransactions(ctx context.Context, accountID int64, transactions []ImportedTransaction) (ImportResult, error) {
	if err := ctx.Err(); err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Imported: len(transactions), Total: len(transactions)}, nil
}

func (s *Memory) Transactions(ctx context.Context, filter TransactionFilter) ([]Transaction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []Transaction{}, nil
}

func (s *Memory) TransactionYears(ctx context.Context) ([]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []int{}, nil
}

func (s *Memory) TransactionSummary(ctx context.Context, filter TransactionFilter) (TransactionSummary, error) {
	if err := ctx.Err(); err != nil {
		return TransactionSummary{}, err
	}
	return TransactionSummary{}, nil
}

func (s *Memory) CategorizeTransaction(ctx context.Context, id int64, input CategorizeTransaction) (CategorizeResult, error) {
	if err := ctx.Err(); err != nil {
		return CategorizeResult{}, err
	}
	return CategorizeResult{Updated: 1}, nil
}

func (s *Memory) CategoryMonthlyTotals(ctx context.Context, filter CategoryReportFilter) ([]MonthlyTotal, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []MonthlyTotal{}, nil
}

func (s *Memory) MainCategoryPeriodTotals(ctx context.Context, filter PeriodReportFilter) ([]CategoryTotal, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []CategoryTotal{}, nil
}

func (s *Memory) SetTransactionHidden(ctx context.Context, id int64, hidden bool) error {
	return ctx.Err()
}

func (s *Memory) CreateCategory(ctx context.Context, input CreateCategory) (Category, error) {
	if err := ctx.Err(); err != nil {
		return Category{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Category{}, fmt.Errorf("name is required")
	}
	kind, purpose, err := normalizeCategoryInput(input.Kind, input.Purpose)
	if err != nil {
		return Category{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	mainExists := false
	for _, mainCategory := range s.mainCategories {
		if mainCategory.ID == input.MainID {
			mainExists = true
			break
		}
	}
	if !mainExists {
		return Category{}, fmt.Errorf("main category not found")
	}

	var nextID int64 = 1
	var sortOrder = 10
	for _, category := range s.categories {
		if category.MainID == input.MainID && strings.EqualFold(category.Name, name) {
			return Category{}, fmt.Errorf("category already exists")
		}
		if category.ID >= nextID {
			nextID = category.ID + 1
		}
		if category.MainID == input.MainID && category.SortOrder >= sortOrder {
			sortOrder = category.SortOrder + 10
		}
	}

	category := Category{ID: nextID, MainID: input.MainID, Name: name, Kind: kind, Purpose: purpose, SortOrder: sortOrder}
	s.categories = append(s.categories, category)
	return category, nil
}

func (s *Memory) UpdateCategory(ctx context.Context, id int64, input UpdateCategory) (Category, error) {
	if err := ctx.Err(); err != nil {
		return Category{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Category{}, fmt.Errorf("name is required")
	}
	kind, purpose, err := normalizeCategoryInput(input.Kind, input.Purpose)
	if err != nil {
		return Category{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	mainExists := false
	for _, mainCategory := range s.mainCategories {
		if mainCategory.ID == input.MainID {
			mainExists = true
			break
		}
	}
	if !mainExists {
		return Category{}, fmt.Errorf("main category not found")
	}

	for _, category := range s.categories {
		if category.ID != id && category.MainID == input.MainID && strings.EqualFold(category.Name, name) {
			return Category{}, fmt.Errorf("category already exists")
		}
	}
	for index, category := range s.categories {
		if category.ID == id {
			s.categories[index].MainID = input.MainID
			s.categories[index].Name = name
			s.categories[index].Kind = kind
			s.categories[index].Purpose = purpose
			return s.categories[index], nil
		}
	}
	return Category{}, fmt.Errorf("category not found")
}

func (s *Memory) DeleteCategory(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	found := false
	categories := s.categories[:0]
	for _, category := range s.categories {
		if category.ID == id {
			found = true
			continue
		}
		categories = append(categories, category)
	}
	if !found {
		return fmt.Errorf("category not found")
	}
	s.categories = categories
	return nil
}

func normalizeCategoryInput(kind string, purpose string) (string, string, error) {
	kind = strings.TrimSpace(kind)
	purpose = strings.TrimSpace(purpose)
	if kind == "" {
		kind = "expense"
	}
	if purpose == "" {
		purpose = "daily"
	}
	if kind != "income" && kind != "expense" && kind != "transfer" {
		return "", "", fmt.Errorf("invalid category kind")
	}
	if purpose != "daily" && purpose != "reservation" && purpose != "investment" {
		return "", "", fmt.Errorf("invalid category purpose")
	}
	return kind, purpose, nil
}

func normalizeAccountInput(input SaveAccount) (Account, error) {
	account := Account{
		Name:        strings.TrimSpace(input.Name),
		IBAN:        strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(input.IBAN), " ", "")),
		Type:        strings.TrimSpace(input.Type),
		Owner:       strings.TrimSpace(input.Owner),
		BankProfile: strings.TrimSpace(input.BankProfile),
	}
	if account.Name == "" {
		return Account{}, fmt.Errorf("name is required")
	}
	if account.Type == "" {
		account.Type = "checking"
	}
	if account.Type != "checking" && account.Type != "savings" {
		return Account{}, fmt.Errorf("invalid account type")
	}
	if account.BankProfile == "" {
		account.BankProfile = "generic"
	}
	return account, nil
}
