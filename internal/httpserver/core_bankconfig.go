package httpserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"hohoho/internal/bankconfig"
)

type saveBankConfigInput struct {
	Slug    string `json:"slug"`
	RawYAML string `json:"rawYaml"`
}

func registerCoreBankConfigRoutes(mux *http.ServeMux, deps CoreDependencies) {
	mux.HandleFunc("GET /api/v1/bank-configs", func(w http.ResponseWriter, r *http.Request) {
		items, err := bankconfig.List(deps.Config.BankConfigDir)
		if err != nil {
			deps.Logger.Error("Bankconfiguraties laden mislukt", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Bankconfiguraties konden niet worden geladen"})
			return
		}
		writeJSON(w, http.StatusOK, items)
	})
	mux.HandleFunc("GET /api/v1/bank-configs/template", func(w http.ResponseWriter, r *http.Request) {
		slug := strings.TrimSpace(r.URL.Query().Get("slug"))
		if slug == "" {
			slug = "nieuwe-bank"
		}
		writeJSON(w, http.StatusOK, map[string]string{"rawYaml": bankconfig.Template(slug)})
	})
	mux.HandleFunc("GET /api/v1/bank-configs/{slug}", func(w http.ResponseWriter, r *http.Request) {
		item, err := bankconfig.Read(deps.Config.BankConfigDir, r.PathValue("slug"))
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, os.ErrNotExist) {
				status = http.StatusNotFound
			}
			writeJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	mux.HandleFunc("PUT /api/v1/bank-configs/{slug}", func(w http.ResponseWriter, r *http.Request) {
		var input saveBankConfigInput
		if !decodeBankConfigInput(w, r, &input) {
			return
		}
		slug := r.PathValue("slug")
		if input.Slug != "" && input.Slug != slug {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Slug in URL en body moeten gelijk zijn"})
			return
		}
		item, err := bankconfig.Save(deps.Config.BankConfigDir, slug, input.RawYAML)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		configuration, err := bankconfig.ConfigurationJSON(item)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Configuratie kon niet worden opgeslagen"})
			return
		}
		profile, err := deps.Store.UpsertBankImportProfile(r.Context(), item.Bank.Name, item.Source.Format, configuration)
		if err != nil {
			deps.Logger.Error("Bankimportprofiel synchroniseren mislukt", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Bankimportprofiel kon niet worden bijgewerkt"})
			return
		}
		item.Status.Profile = profile.ID
		writeJSON(w, http.StatusOK, item)
	})
	mux.HandleFunc("DELETE /api/v1/bank-configs/{slug}", func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		item, readErr := bankconfig.Read(deps.Config.BankConfigDir, slug)
		if err := bankconfig.Delete(deps.Config.BankConfigDir, slug); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if readErr == nil {
			_ = deps.Store.DeleteBankImportProfileByName(r.Context(), item.Bank.Name)
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func decodeBankConfigInput(w http.ResponseWriter, r *http.Request, input *saveBankConfigInput) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Ongeldige JSON-invoer"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Verwacht één JSON-object"})
		return false
	}
	return true
}
