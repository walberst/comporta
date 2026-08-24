// Pacote api concentra os handlers HTTP do gateway: o roteamento
// administrativo (CRUD de parceiros, rotas e politicas) e o handler de proxy
// que aplica autenticacao, rate limiting e auditoria antes de encaminhar a
// requisicao ao upstream.
package api

import (
	"encoding/json"
	"net/http"
)

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, apiError{Error: message})
}

type pagedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	Total      int         `json:"total"`
	TotalPages int         `json:"total_pages"`
}

func newPagedResponse(data interface{}, page, pageSize, total int) pagedResponse {
	totalPages := 0
	if pageSize > 0 {
		totalPages = (total + pageSize - 1) / pageSize
	}
	return pagedResponse{
		Data:       data,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}
}
