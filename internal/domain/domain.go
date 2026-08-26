// Package domain contem as entidades centrais do gateway. Nenhuma dependencia
// de infraestrutura (banco, redis, http) deve vazar para dentro deste pacote,
// assim ele fica facil de testar isoladamente e serve de contrato para os
// repositorios em internal/store.
package domain

import "time"

// Partner e uma empresa parceira autorizada a consumir o gateway. A APIKey e
// o unico segredo de autenticacao usado nas requisicoes de proxy.
type Partner struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	APIKey    string    `json:"api_key"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

// Route mapeia um prefixo de path publico para um servico upstream interno.
// O prefixo mais especifico (mais longo) que casar com o path da requisicao
// vence, permitindo rotas genericas com excecoes mais finas.
type Route struct {
	ID          int64     `json:"id"`
	PathPrefix  string    `json:"path_prefix"`
	UpstreamURL string    `json:"upstream_url"`
	StripPrefix bool      `json:"strip_prefix"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

// RateLimitPolicy define quantas requisicoes um parceiro pode fazer por
// minuto em uma rota especifica. RouteID igual a zero representa uma politica
// padrao aplicada a qualquer rota que o parceiro acesse sem regra dedicada.
type RateLimitPolicy struct {
	ID                int64     `json:"id"`
	PartnerID         int64     `json:"partner_id"`
	RouteID           int64     `json:"route_id"`
	RequestsPerMinute int       `json:"requests_per_minute"`
	CreatedAt         time.Time `json:"created_at"`
}

// AuditLog registra cada requisicao que passou (ou tentou passar) pelo
// gateway, usado tanto para auditoria quanto para os paineis de consumo.
type AuditLog struct {
	ID         int64     `json:"id"`
	PartnerID  int64     `json:"partner_id"`
	RouteID    int64     `json:"route_id"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	StatusCode int       `json:"status_code"`
	LatencyMs  int64     `json:"latency_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

// Pagination e o envelope padrao usado pelos endpoints administrativos que
// retornam listas.
type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

// NormalizePageParams aplica limites sensatos de paginacao evitando que um
// cliente peca page_size absurdo e derrube o banco com uma consulta gigante.
func NormalizePageParams(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
