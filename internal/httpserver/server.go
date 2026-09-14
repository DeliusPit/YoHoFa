package httpserver

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"geldzaken/internal/bankimport"
	"geldzaken/internal/config"
	"geldzaken/internal/objectstore"
	"geldzaken/internal/store"
	"geldzaken/internal/systembackup"
)

type Dependencies struct {
	Config              config.Config
	Store               store.Backend
	Documents           store.DocumentRepository
	Assets              store.AssetRepository
	Liabilities         store.LiabilityRepository
	Cases               store.CaseRepository
	Relations           store.EntityRelationRepository
	Events              store.DomainEventRepository
	Objects             objectstore.Store
	ObjectStoreRequired bool
	Configuration       *config.Manager
	SystemBackup        *systembackup.Service
	Logger              *slog.Logger
}

func New(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	registerConfigurationRoutes(mux, deps)
	registerSystemBackupRoutes(mux, deps)

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := deps.Store.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
			return
		}
		if deps.ObjectStoreRequired && deps.Objects == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
			return
		}
		if deps.Objects != nil {
			if err := deps.Objects.Ping(r.Context()); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy"})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	documentsAvailable := deps.Documents != nil && deps.Objects != nil
	if documentsAvailable {
		registerDocumentRoutes(mux, deps)
	} else {
		registerUnavailableDocumentRoutes(mux)
	}
	if deps.Assets != nil && deps.Relations != nil {
		registerAssetRoutes(mux, deps)
		if documentsAvailable {
			registerAssetDocumentRoutes(mux, deps)
		} else {
			registerUnavailableObjectDocumentRoutes(mux, "assets")
		}
	} else {
		unavailable := func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "assets_require_postgres"})
		}
		mux.HandleFunc("/api/v1/assets", unavailable)
		mux.HandleFunc("/api/v1/assets/", unavailable)
	}
	if deps.Liabilities != nil && deps.Relations != nil {
		registerLiabilityRoutes(mux, deps)
		if documentsAvailable {
			registerLiabilityDocumentRoutes(mux, deps)
		} else {
			registerUnavailableObjectDocumentRoutes(mux, "liabilities")
		}
	} else {
		unavailable := func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "liabilities_require_postgres"})
		}
		mux.HandleFunc("/api/v1/liabilities", unavailable)
		mux.HandleFunc("/api/v1/liabilities/", unavailable)
	}
	if deps.Cases != nil && deps.Relations != nil {
		registerCaseRoutes(mux, deps)
		if documentsAvailable {
			registerCaseDocumentRoutes(mux, deps)
		} else {
			registerUnavailableObjectDocumentRoutes(mux, "cases")
		}
	} else {
		unavailable := func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "cases_require_postgres"})
		}
		mux.HandleFunc("/api/v1/cases", unavailable)
		mux.HandleFunc("/api/v1/cases/", unavailable)
	}
	if deps.Events != nil {
		registerDomainEventRoutes(mux, deps)
	} else {
		registerUnavailableDomainEventRoutes(mux)
	}

	if deps.Config.WebDir != "" {
		mux.HandleFunc("GET /", staticHandler(deps.Config.WebDir))
	}

	mux.HandleFunc("GET /api/v1/summary", func(w http.ResponseWriter, r *http.Request) {
		summary, err := deps.Store.Summary(r.Context())
		if err != nil {
			deps.Logger.Error("summary", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, summary)
	})

	mux.HandleFunc("GET /api/v1/categories", func(w http.ResponseWriter, r *http.Request) {
		categories, err := deps.Store.Categories(r.Context())
		if err != nil {
			deps.Logger.Error("categories", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, categories)
	})

	mux.HandleFunc("GET /api/v1/accounts", func(w http.ResponseWriter, r *http.Request) {
		accounts, err := deps.Store.Accounts(r.Context())
		if err != nil {
			deps.Logger.Error("accounts", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, accounts)
	})

	mux.HandleFunc("GET /api/v1/transactions", func(w http.ResponseWriter, r *http.Request) {
		transactions, err := deps.Store.Transactions(r.Context(), transactionFilterFromRequest(r))
		if err != nil {
			deps.Logger.Error("transactions", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, transactions)
	})

	mux.HandleFunc("POST /api/v1/transactions", func(w http.ResponseWriter, r *http.Request) {
		repository, ok := deps.Store.(store.ManualTransactionRepository)
		if !ok {
			writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "manual_transactions_unavailable"})
			return
		}
		var input store.CreateManualTransaction
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		transaction, err := repository.CreateManualTransaction(r.Context(), input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, transaction)
	})

	mux.HandleFunc("GET /api/v1/transactions/years", func(w http.ResponseWriter, r *http.Request) {
		years, err := deps.Store.TransactionYears(r.Context())
		if err != nil {
			deps.Logger.Error("transaction years", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, years)
	})

	mux.HandleFunc("GET /api/v1/transactions/summary", func(w http.ResponseWriter, r *http.Request) {
		summary, err := deps.Store.TransactionSummary(r.Context(), transactionFilterFromRequest(r))
		if err != nil {
			deps.Logger.Error("transaction summary", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, summary)
	})

	mux.HandleFunc("PATCH /api/v1/transactions/{id}/hidden", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		if err := deps.Store.SetTransactionHidden(r.Context(), id, true); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "hidden"})
	})

	mux.HandleFunc("DELETE /api/v1/transactions/{id}/hidden", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		if err := deps.Store.SetTransactionHidden(r.Context(), id, false); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "visible"})
	})

	mux.HandleFunc("GET /api/v1/reports/category-monthly", func(w http.ResponseWriter, r *http.Request) {
		mainCategoryID, _ := strconv.ParseInt(r.URL.Query().Get("mainCategoryId"), 10, 64)
		categoryID, _ := strconv.ParseInt(r.URL.Query().Get("categoryId"), 10, 64)
		totals, err := deps.Store.CategoryMonthlyTotals(r.Context(), store.CategoryReportFilter{MainCategoryID: mainCategoryID, CategoryID: categoryID})
		if err != nil {
			deps.Logger.Error("category monthly report", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, totals)
	})

	mux.HandleFunc("GET /api/v1/reports/main-category-period", func(w http.ResponseWriter, r *http.Request) {
		mainCategoryID, _ := strconv.ParseInt(r.URL.Query().Get("mainCategoryId"), 10, 64)
		totals, err := deps.Store.MainCategoryPeriodTotals(r.Context(), store.PeriodReportFilter{
			MainCategoryID: mainCategoryID,
			From:           r.URL.Query().Get("from"),
			To:             r.URL.Query().Get("to"),
		})
		if err != nil {
			deps.Logger.Error("main category period report", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, totals)
	})

	mux.HandleFunc("PATCH /api/v1/transactions/{id}/category", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		var input store.CategorizeTransaction
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		result, err := deps.Store.CategorizeTransaction(r.Context(), id, input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /api/v1/accounts", func(w http.ResponseWriter, r *http.Request) {
		var input store.SaveAccount
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		account, err := deps.Store.CreateAccount(r.Context(), input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, account)
	})

	mux.HandleFunc("PATCH /api/v1/accounts/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		var input store.SaveAccount
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		account, err := deps.Store.UpdateAccount(r.Context(), id, input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, account)
	})

	mux.HandleFunc("DELETE /api/v1/accounts/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		if err := deps.Store.DeleteAccount(r.Context(), id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/v1/main-categories", func(w http.ResponseWriter, r *http.Request) {
		mainCategories, err := deps.Store.MainCategories(r.Context())
		if err != nil {
			deps.Logger.Error("main categories", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, mainCategories)
	})

	mux.HandleFunc("POST /api/v1/main-categories", func(w http.ResponseWriter, r *http.Request) {
		var input store.CreateMainCategory
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		if strings.TrimSpace(input.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name_is_required"})
			return
		}

		mainCategory, err := deps.Store.CreateMainCategory(r.Context(), input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, mainCategory)
	})

	mux.HandleFunc("PATCH /api/v1/main-categories/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}

		var input store.UpdateMainCategory
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		if strings.TrimSpace(input.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name_is_required"})
			return
		}

		mainCategory, err := deps.Store.UpdateMainCategory(r.Context(), id, input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, mainCategory)
	})

	mux.HandleFunc("DELETE /api/v1/main-categories/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		if err := deps.Store.DeleteMainCategory(r.Context(), id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /api/v1/categories", func(w http.ResponseWriter, r *http.Request) {
		var input store.CreateCategory
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		if strings.TrimSpace(input.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name_is_required"})
			return
		}

		category, err := deps.Store.CreateCategory(r.Context(), input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, category)
	})

	mux.HandleFunc("PATCH /api/v1/categories/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}

		var input store.UpdateCategory
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		if strings.TrimSpace(input.Name) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name_is_required"})
			return
		}

		category, err := deps.Store.UpdateCategory(r.Context(), id, input)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, category)
	})

	mux.HandleFunc("DELETE /api/v1/categories/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_id"})
			return
		}
		if err := deps.Store.DeleteCategory(r.Context(), id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/v1/imports/bank-profiles", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string][]string{"profiles": bankimport.ProfileNames()})
	})

	mux.HandleFunc("POST /api/v1/imports/preview", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(13 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_multipart_form"})
			return
		}

		profile := r.FormValue("profile")
		if profile == "" {
			profile = "generic"
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing_file"})
			return
		}
		defer file.Close()

		preview, err := bankimport.PreviewFile(profile, header.Filename, file)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, preview)
	})

	mux.HandleFunc("POST /api/v1/imports", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(13 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_multipart_form"})
			return
		}

		accountID, err := strconv.ParseInt(r.FormValue("accountId"), 10, 64)
		if err != nil || accountID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_account"})
			return
		}

		account, err := accountByID(r.Context(), deps.Store, accountID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing_file"})
			return
		}
		defer file.Close()

		preview, err := bankimport.PreviewFile(account.BankProfile, header.Filename, file)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		result, err := deps.Store.ImportTransactions(r.Context(), account.ID, importedTransactions(account.ID, preview.Transactions))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"profile": preview.Profile, "account": account, "result": result, "warnings": preview.Warnings})
	})

	var handler http.Handler = mux
	if deps.Config.AuthUsername != "" && deps.Config.AuthPassword != "" {
		handler = basicAuth(deps.Config.AuthUsername, deps.Config.AuthPassword, handler)
	}
	return cors(deps.Config.CORSOrigin, handler)
}

func registerUnavailableDocumentRoutes(mux *http.ServeMux) {
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "documents_require_postgres_and_minio"})
	}
	mux.HandleFunc("GET /api/v1/documents", unavailable)
	mux.HandleFunc("POST /api/v1/documents", unavailable)
	mux.HandleFunc("GET /api/v1/documents/{id}/content", unavailable)
	mux.HandleFunc("GET /api/v1/transactions/{id}/documents", unavailable)
	mux.HandleFunc("POST /api/v1/transactions/{id}/documents", unavailable)
	mux.HandleFunc("DELETE /api/v1/transactions/{id}/documents/{documentId}", unavailable)
}

func registerUnavailableObjectDocumentRoutes(mux *http.ServeMux, objectPath string) {
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "documents_require_postgres_and_minio"})
	}
	mux.HandleFunc("GET /api/v1/"+objectPath+"/{id}/documents", unavailable)
	mux.HandleFunc("POST /api/v1/"+objectPath+"/{id}/documents", unavailable)
	mux.HandleFunc("DELETE /api/v1/"+objectPath+"/{id}/documents/{documentId}", unavailable)
}

func basicAuth(username string, password string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providedUsername, providedPassword, ok := r.BasicAuth()
		usernameMatches := subtle.ConstantTimeCompare([]byte(providedUsername), []byte(username)) == 1
		passwordMatches := subtle.ConstantTimeCompare([]byte(providedPassword), []byte(password)) == 1
		if !ok || !usernameMatches || !passwordMatches {
			w.Header().Set("WWW-Authenticate", `Basic realm="Geldzaken", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func cors(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func staticHandler(webDir string) http.HandlerFunc {
	files := http.FileServer(http.Dir(webDir))
	return func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(r.URL.Path)
		if path == "." || path == "/" {
			http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
			return
		}

		filePath := filepath.Join(webDir, strings.TrimPrefix(path, "/"))
		if _, err := os.Stat(filePath); err != nil {
			http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
			return
		}
		files.ServeHTTP(w, r)
	}
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func transactionFilterFromRequest(r *http.Request) store.TransactionFilter {
	accountID, _ := strconv.ParseInt(r.URL.Query().Get("accountId"), 10, 64)
	mainCategoryID, _ := strconv.ParseInt(r.URL.Query().Get("mainCategoryId"), 10, 64)
	categoryID, _ := strconv.ParseInt(r.URL.Query().Get("categoryId"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	return store.TransactionFilter{
		AccountID:      accountID,
		MainCategoryID: mainCategoryID,
		CategoryID:     categoryID,
		From:           r.URL.Query().Get("from"),
		To:             r.URL.Query().Get("to"),
		Search:         r.URL.Query().Get("search"),
		Uncategorized:  r.URL.Query().Get("uncategorized") == "true",
		HiddenOnly:     r.URL.Query().Get("hidden") == "true",
		Limit:          limit,
		Offset:         offset,
	}
}

func accountByID(ctx context.Context, backend store.Backend, id int64) (store.Account, error) {
	accounts, err := backend.Accounts(ctx)
	if err != nil {
		return store.Account{}, err
	}
	for _, account := range accounts {
		if account.ID == id {
			return account, nil
		}
	}
	return store.Account{}, fmt.Errorf("account not found")
}

func importedTransactions(accountID int64, transactions []bankimport.Transaction) []store.ImportedTransaction {
	imported := make([]store.ImportedTransaction, 0, len(transactions))
	occurrences := make(map[string]int)
	for _, transaction := range transactions {
		item := store.ImportedTransaction{
			BookedOn:          transaction.BookedOn,
			Description:       transaction.Description,
			Counterparty:      transaction.Counterparty,
			CounterpartyIBAN:  transaction.CounterpartyIBAN,
			AmountCents:       transaction.AmountCents,
			ExternalReference: transaction.ExternalReference,
		}
		baseFingerprint := importFingerprint(accountID, item)
		occurrences[baseFingerprint]++
		item.ImportFingerprint = fingerprintOccurrence(baseFingerprint, occurrences[baseFingerprint])
		imported = append(imported, item)
	}
	return imported
}

func fingerprintOccurrence(baseFingerprint string, occurrence int) string {
	if occurrence <= 1 {
		return baseFingerprint
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s|%d", baseFingerprint, occurrence)))
	return hex.EncodeToString(hash[:])
}

func importFingerprint(accountID int64, transaction store.ImportedTransaction) string {
	data := fmt.Sprintf("%d|%s|%d|%s|%s|%s", accountID, transaction.BookedOn, transaction.AmountCents, transaction.Description, transaction.CounterpartyIBAN, transaction.ExternalReference)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
