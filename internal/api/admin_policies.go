package api

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/hagile/comporta/internal/domain"
)

type createPolicyRequest struct {
	PartnerID         int64 `json:"partner_id"`
	RouteID           int64 `json:"route_id"`
	RequestsPerMinute int   `json:"requests_per_minute"`
}

func (h *AdminHandler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	var req createPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corpo da requisicao invalido")
		return
	}
	if err := requirePositive("requests_per_minute", req.RequestsPerMinute); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.PartnerID <= 0 {
		writeError(w, http.StatusBadRequest, "partner_id e obrigatorio")
		return
	}
	if _, err := h.Repos.Partners.GetByID(r.Context(), req.PartnerID); err != nil {
		writeError(w, http.StatusBadRequest, "partner_id nao corresponde a um parceiro existente")
		return
	}
	// route_id igual a zero e valido: representa a politica padrao do
	// parceiro para qualquer rota sem regra dedicada.
	if req.RouteID != 0 {
		if _, err := h.Repos.Routes.GetByID(r.Context(), req.RouteID); err != nil {
			writeError(w, http.StatusBadRequest, "route_id nao corresponde a uma rota existente")
			return
		}
	}

	policy := domain.RateLimitPolicy{
		PartnerID:         req.PartnerID,
		RouteID:           req.RouteID,
		RequestsPerMinute: req.RequestsPerMinute,
	}
	if err := h.Repos.Policies.Create(r.Context(), &policy); err != nil {
		h.Log.Error("falha ao criar policy", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "falha ao criar politica de rate limit")
		return
	}
	writeJSON(w, http.StatusCreated, policy)
}

func (h *AdminHandler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	policies, total, err := h.Repos.Policies.List(r.Context(), page, pageSize)
	if err != nil {
		h.Log.Error("falha ao listar policies", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "falha ao listar politicas")
		return
	}
	writeJSON(w, http.StatusOK, newPagedResponse(policies, page, pageSize, total))
}

func (h *AdminHandler) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}
	if err := h.Repos.Policies.Delete(r.Context(), id); err != nil {
		respondLookupError(w, err, "politica")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	logs, total, err := h.Repos.Audit.List(r.Context(), page, pageSize)
	if err != nil {
		h.Log.Error("falha ao listar audit logs", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "falha ao listar logs de auditoria")
		return
	}
	writeJSON(w, http.StatusOK, newPagedResponse(logs, page, pageSize, total))
}
