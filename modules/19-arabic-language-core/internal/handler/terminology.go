package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/operan/arabic-language-core/internal/clients"
	"github.com/operan/arabic-language-core/internal/events"
	"github.com/operan/arabic-language-core/internal/store"
	"github.com/operan/arabic-language-core/internal/ctxkeys"
)

// CheckTerminologyRequest is the request body for POST /v1/terminology/check.
type CheckTerminologyRequest struct {
	Text     string `json:"text"`
	Category string `json:"category,omitempty"`
	Domain   string `json:"domain,omitempty"`
}

// CheckTerminologyResponse is the response for POST /v1/terminology/check.
type CheckTerminologyResponse struct {
	MatchedTerms []MatchedTerm `json:"matched_terms"`
	FlaggedTerms []FlaggedTerm `json:"flagged_terms"`
	CheckCount   int           `json:"check_count"`
	FlaggedCount int           `json:"flagged_count"`
}

// MatchedTerm represents a term found in the glossary.
type MatchedTerm struct {
	Term       string `json:"term"`
	Category   string `json:"category"`
	IsApproved bool   `json:"is_approved"`
}

// FlaggedTerm represents a term that is unapproved/deprecated.
type FlaggedTerm struct {
	Term       string `json:"term"`
	Reason     string `json:"reason"`
	Suggestion string `json:"suggestion,omitempty"`
}

// TerminologyHandler handles terminology endpoints.
type TerminologyHandler struct {
	store     *store.TerminologyStore
	broker    *events.Broker
	m12       *clients.M12Client
	jwtSecret string
	logger    Logger
}

// NewTerminologyHandler creates a terminology handler.
func NewTerminologyHandler(s *store.TerminologyStore, b *events.Broker, m *clients.M12Client, jwt string, l Logger) *TerminologyHandler {
	return &TerminologyHandler{store: s, broker: b, m12: m, jwtSecret: jwt, logger: l}
}

// HandleCheck checks text against the terminology glossary.
func (h *TerminologyHandler) HandleCheck(w http.ResponseWriter, r *http.Request) {
	var req CheckTerminologyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)
	terms, err := h.store.CheckTerms(r.Context(), tenantID, req.Text, req.Category, req.Domain)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("query glossary: %v", err)})
		return
	}

	matchedTerms := make([]MatchedTerm, 0)
	flaggedTerms := make([]FlaggedTerm, 0)

	for _, term := range terms {
		// Simple substring check
		if contains(req.Text, term.TermArabic) {
			if term.Status == "active" {
				matchedTerms = append(matchedTerms, MatchedTerm{
					Term:       term.TermArabic,
					Category:   term.Category,
					IsApproved: true,
				})
			} else {
				suggestion := term.PreferredForm
				if suggestion == "" {
					suggestion = "review required"
				}
				flaggedTerms = append(flaggedTerms, FlaggedTerm{
					Term:       term.TermArabic,
					Reason:     fmt.Sprintf("term_%s", term.Status),
					Suggestion: suggestion,
				})
				if h.broker != nil {
					h.broker.TerminologyViolation(r.Context(), tenantID, term.TermArabic,
						string(fmt.Sprintf("term_%s", term.Status)), suggestion, userID(r.Context()))
				}
			}
		}

		for _, alt := range term.Alternatives {
			if contains(req.Text, alt) {
				if term.Status == "active" {
					matchedTerms = append(matchedTerms, MatchedTerm{
						Term:       alt,
						Category:   term.Category,
						IsApproved: true,
					})
				} else {
					flaggedTerms = append(flaggedTerms, FlaggedTerm{
						Term:       alt,
						Reason:     fmt.Sprintf("variant_of_%s", term.TermArabic),
						Suggestion: term.PreferredForm,
					})
				}
			}
		}
	}

	if h.broker != nil {
		h.broker.TerminologyCheck(r.Context(), tenantID, len(req.Text), len(matchedTerms), len(flaggedTerms))
	}

	if h.store != nil {
		_ = h.store.LogUsage(r.Context(), &store.TerminologyUsageLog{
			TenantID:     tenantID,
			SourceText:   req.Text,
			MatchedTerms: store.SliceToJSONByte(matchedTerms),
			FlaggedTerms: store.SliceToJSONByte(flaggedTerms),
			CheckedBy:    userID(r.Context()),
		})
	}

	writeJSON(w, http.StatusOK, CheckTerminologyResponse{
		MatchedTerms: matchedTerms,
		FlaggedTerms: flaggedTerms,
		CheckCount:   len(matchedTerms),
		FlaggedCount: len(flaggedTerms),
	})
}

// HandleListGlossary returns paginated glossary entries.
func (h *TerminologyHandler) HandleListGlossary(w http.ResponseWriter, r *http.Request) {
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	category := r.URL.Query().Get("category")
	domain := r.URL.Query().Get("domain")
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page < 1 {
		page = 1
	}

	terms, total, err := h.store.List(r.Context(), tenantID, category, domain, status, page, pageSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("list glossary: %v", err)})
		return
	}

	type listEntry struct {
		ID           string   `json:"id"`
		TermArabic   string   `json:"term_arabic"`
		TermEnglish  string   `json:"term_english,omitempty"`
		Category     string   `json:"category"`
		Domain       string   `json:"domain"`
		Status       string   `json:"status"`
		Alternatives []string `json:"alternatives"`
		CreatedAt    string   `json:"created_at"`
	}

	entries := make([]listEntry, 0, len(terms))
	for _, t := range terms {
		entries = append(entries, listEntry{
			ID:           t.ID,
			TermArabic:   t.TermArabic,
			TermEnglish:  t.TermEnglish,
			Category:     t.Category,
			Domain:       t.Domain,
			Status:       t.Status,
			Alternatives: t.Alternatives,
			CreatedAt:    t.CreatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"terms":     entries,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	})
}

// HandleCreateGlossary adds a new glossary entry.
func (h *TerminologyHandler) HandleCreateGlossary(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TermArabic     string   `json:"term_arabic"`
		TermEnglish    string   `json:"term_english"`
		Category       string   `json:"category"`
		Domain         string   `json:"domain"`
		PreferredForm  string   `json:"preferred_form,omitempty"`
		Alternatives   []string `json:"alternatives"`
		Notes          string   `json:"notes"`
		ApprovedBy     string   `json:"approved_by,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	entry := &store.TerminologyGlossary{
		ID:               uuid.New().String(),
		TenantID:         tenantID,
		TermArabic:       req.TermArabic,
		TermEnglish:      req.TermEnglish,
		Category:         req.Category,
		Domain:           req.Domain,
		Status:           "active",
		ApprovedBy:       req.ApprovedBy,
		PreferredForm:    req.PreferredForm,
		Alternatives:     req.Alternatives,
		Notes:            req.Notes,
	}

	if err := h.store.Create(r.Context(), entry); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("create term: %v", err)})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"term": entry})
}

// HandleUpdateGlossary updates an existing glossary entry.
func (h *TerminologyHandler) HandleUpdateGlossary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	_, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "term not found"})
		return
	}

	var req struct {
		TermEnglish   *string  `json:"term_english"`
		Category      *string  `json:"category"`
		Domain        *string  `json:"domain"`
		Status        *string  `json:"status"`
		PreferredForm *string  `json:"preferred_form"`
		Alternatives  []string `json:"alternatives"`
		Notes         *string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	entry := &store.TerminologyGlossary{
		ID:           id,
		TenantID:     tenantID,
		Alternatives: req.Alternatives,
	}
	if req.TermEnglish != nil {
		entry.TermEnglish = *req.TermEnglish
	}
	if req.Category != nil {
		entry.Category = *req.Category
	}
	if req.Domain != nil {
		entry.Domain = *req.Domain
	}
	if req.Status != nil {
		entry.Status = *req.Status
	}
	if req.PreferredForm != nil {
		entry.PreferredForm = *req.PreferredForm
	}
	if req.Notes != nil {
		entry.Notes = *req.Notes
	}

	if err := h.store.Update(r.Context(), entry); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("update term: %v", err)})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"term": entry})
}

// HandleDeleteGlossary removes a glossary entry.
func (h *TerminologyHandler) HandleDeleteGlossary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tenantID := r.Context().Value(ctxkeys.TenantIDKey).(string)

	if err := h.store.Delete(r.Context(), id, tenantID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "term not found or not authorized"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}