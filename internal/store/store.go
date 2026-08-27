// Package store define os contratos de persistencia do gateway e duas
// implementacoes: oracle.go (producao, via go-ora) e memory.go (fake em
// memoria usada nos testes unitarios, ja que subir um Oracle real a cada
// execucao de teste seria lento demais para rodar no CI a cada push).
package store

import (
	"context"
	"errors"
	"time"

	"github.com/walberst/comporta/internal/domain"
)

// ErrNotFound e retornado por qualquer repositorio quando o registro buscado
// nao existe, permitindo que a camada HTTP decida o codigo de status certo
// (404) sem conhecer detalhes do driver de banco.
var ErrNotFound = errors.New("registro nao encontrado")

// ConsumerStat resume o consumo de um parceiro em uma rota, usado no ranking
// de top consumidores exibido no painel.
type ConsumerStat struct {
	PartnerID   int64  `json:"partner_id"`
	PartnerName string `json:"partner_name"`
	RouteID     int64  `json:"route_id"`
	RoutePath   string `json:"route_path"`
	Requests    int64  `json:"requests"`
}

type PartnerRepository interface {
	Create(ctx context.Context, p *domain.Partner) error
	GetByID(ctx context.Context, id int64) (*domain.Partner, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*domain.Partner, error)
	List(ctx context.Context, page, pageSize int) ([]domain.Partner, int, error)
	Update(ctx context.Context, p *domain.Partner) error
	Delete(ctx context.Context, id int64) error
}

type RouteRepository interface {
	Create(ctx context.Context, r *domain.Route) error
	GetByID(ctx context.Context, id int64) (*domain.Route, error)
	List(ctx context.Context, page, pageSize int) ([]domain.Route, int, error)
	ListActive(ctx context.Context) ([]domain.Route, error)
	Update(ctx context.Context, r *domain.Route) error
	Delete(ctx context.Context, id int64) error
}

type PolicyRepository interface {
	Create(ctx context.Context, pol *domain.RateLimitPolicy) error
	List(ctx context.Context, page, pageSize int) ([]domain.RateLimitPolicy, int, error)
	// FindEffective retorna a politica mais especifica para o par parceiro/rota,
	// caindo para uma politica com RouteID zero (padrao do parceiro) se existir.
	FindEffective(ctx context.Context, partnerID, routeID int64) (*domain.RateLimitPolicy, error)
	Delete(ctx context.Context, id int64) error
}

type AuditRepository interface {
	Insert(ctx context.Context, log *domain.AuditLog) error
	List(ctx context.Context, page, pageSize int) ([]domain.AuditLog, int, error)
	TopConsumers(ctx context.Context, since time.Time, limit int) ([]ConsumerStat, error)
}

// Repositories agrupa todos os repositorios para facilitar a injecao de
// dependencia nos handlers HTTP e no motor de proxy.
type Repositories struct {
	Partners PartnerRepository
	Routes   RouteRepository
	Policies PolicyRepository
	Audit    AuditRepository
}
