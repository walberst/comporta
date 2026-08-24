package api

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/hagile/comporta/internal/domain"
)

type createRouteRequest struct {
	PathPrefix  string `json:"path_prefix"`
	UpstreamURL string `json:"upstream_url"`
	StripPrefix bool   `json:"strip_prefix"`
}

func (h *AdminHandler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	var req createRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corpo da requisicao invalido")
		return
	}
	if err := requirePathPrefix(req.PathPrefix); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := requireValidURL(req.UpstreamURL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	route := domain.Route{
		PathPrefix:  req.PathPrefix,
		UpstreamURL: req.UpstreamURL,
		StripPrefix: req.StripPrefix,
		Active:      true,
	}
	if err := h.Repos.Routes.Create(r.Context(), &route); err != nil {
		h.Log.Error("falha ao criar route", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "falha ao criar rota")
		return
	}
	writeJSON(w, http.StatusCreated, route)
}

func (h *AdminHandler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pageParams(r)
	routes, total, err := h.Repos.Routes.List(r.Context(), page, pageSize)
	if err != nil {
		h.Log.Error("falha ao listar routes", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "falha ao listar rotas")
		return
	}
	writeJSON(w, http.StatusOK, newPagedResponse(routes, page, pageSize, total))
}

func (h *AdminHandler) GetRoute(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}
	route, err := h.Repos.Routes.GetByID(r.Context(), id)
	if err != nil {
		respondLookupError(w, err, "rota")
		return
	}
	writeJSON(w, http.StatusOK, route)
}

type updateRouteRequest struct {
	UpstreamURL string `json:"upstream_url"`
	StripPrefix *bool  `json:"strip_prefix"`
	Active      *bool  `json:"active"`
}

func (h *AdminHandler) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}
	existing, err := h.Repos.Routes.GetByID(r.Context(), id)
	if err != nil {
		respondLookupError(w, err, "rota")
		return
	}

	var req updateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "corpo da requisicao invalido")
		return
	}
	if req.UpstreamURL != "" {
		if err := requireValidURL(req.UpstreamURL); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		existing.UpstreamURL = req.UpstreamURL
	}
	if req.StripPrefix != nil {
		existing.StripPrefix = *req.StripPrefix
	}
	if req.Active != nil {
		existing.Active = *req.Active
	}

	if err := h.Repos.Routes.Update(r.Context(), existing); err != nil {
		respondLookupError(w, err, "rota")
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (h *AdminHandler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "id invalido")
		return
	}
	if err := h.Repos.Routes.Delete(r.Context(), id); err != nil {
		respondLookupError(w, err, "rota")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
