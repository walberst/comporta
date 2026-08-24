package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"go.uber.org/zap"

	"github.com/hagile/comporta/internal/domain"
	"github.com/hagile/comporta/internal/store"
)

// AdminHandler expoe os endpoints REST de cadastro usados pelo painel e por
// operacao manual (ex: onboarding de um novo parceiro).
type AdminHandler struct {
	Repos store.Repositories
	Log   *zap.Logger
}

type createPartnerRequest struct {
	Name string `json:"name"`
}

type partnerResponse struct {
	domain.Partner
}

func (h *AdminHandler) CreatePartner(w http.ResponseWriter, r *http.Request) {
	var req createPartnerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corpo da requisicao invalido")
		return
	}
	if err := requireNonEmpty("name", req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	apiKey, err := generateAPIKey()
	if err != nil {
		h.Log.Error("falha ao gerar api key", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "falha ao gerar api key")
		return
	}

	partner := domain.Partner{Name: req.Name, APIKey: apiKey, Active: true}
	if err := h.Repos.Partners.Create(r.Context(), &partner); err != nil {
		h.Log.Error("falha ao criar partner", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "falha ao criar parceiro")
		return
	}
	writeJSON(w, http.StatusCreated, partnerResponse{partner})
}

func (h *AdminHandler) ListPartners(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	partners, total, err := h.Repos.Partners.List(r.Context(), page, pageSize)
	if err != nil {
		h.Log.Error("falha ao listar partners", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "falha ao listar parceiros")
		return
	}
	writeJSON(w, http.StatusOK, newPagedResponse(partners, page, pageSize, total))
}

func (h *AdminHandler) GetPartner(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}
	partner, err := h.Repos.Partners.GetByID(r.Context(), id)
	if err != nil {
		respondLookupError(w, err, "parceiro")
		return
	}
	writeJSON(w, http.StatusOK, partnerResponse{*partner})
}

type updatePartnerRequest struct {
	Name   string `json:"name"`
	Active *bool  `json:"active"`
}

func (h *AdminHandler) UpdatePartner(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}
	existing, err := h.Repos.Partners.GetByID(r.Context(), id)
	if err != nil {
		respondLookupError(w, err, "parceiro")
		return
	}

	var req updatePartnerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corpo da requisicao invalido")
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Active != nil {
		existing.Active = *req.Active
	}

	if err := h.Repos.Partners.Update(r.Context(), existing); err != nil {
		respondLookupError(w, err, "parceiro")
		return
	}
	writeJSON(w, http.StatusOK, partnerResponse{*existing})
}

func (h *AdminHandler) DeletePartner(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}
	if err := h.Repos.Partners.Delete(r.Context(), id); err != nil {
		respondLookupError(w, err, "parceiro")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func pageParams(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	return domain.NormalizePageParams(page, pageSize)
}

func parseID(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}

func respondLookupError(w http.ResponseWriter, err error, entity string) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, entity+" nao encontrado")
		return
	}
	writeError(w, http.StatusInternalServerError, "falha inesperada ao acessar "+entity)
}
