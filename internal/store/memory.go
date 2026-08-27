package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/walberst/comporta/internal/domain"
)

// MemoryRepositories e uma implementacao em memoria dos repositorios, usada
// nos testes unitarios e no modo de desenvolvimento local sem Oracle. Nao usar
// em producao: nao persiste nada e nao e distribuivel entre instancias.
type MemoryRepositories struct {
	mu sync.RWMutex

	partners map[int64]domain.Partner
	routes   map[int64]domain.Route
	policies map[int64]domain.RateLimitPolicy
	audit    []domain.AuditLog

	nextPartnerID int64
	nextRouteID   int64
	nextPolicyID  int64
	nextAuditID   int64
}

// NewMemoryRepositories cria o conjunto de repositorios em memoria ja
// prontos para uso, sem nenhum dado de exemplo (o seed e sempre um passo
// explicito, mesmo neste modo).
func NewMemoryRepositories() *MemoryRepositories {
	return &MemoryRepositories{
		partners: make(map[int64]domain.Partner),
		routes:   make(map[int64]domain.Route),
		policies: make(map[int64]domain.RateLimitPolicy),
	}
}

func (m *MemoryRepositories) AsRepositories() Repositories {
	return Repositories{
		Partners: (*memoryPartners)(m),
		Routes:   (*memoryRoutes)(m),
		Policies: (*memoryPolicies)(m),
		Audit:    (*memoryAudit)(m),
	}
}

type memoryPartners MemoryRepositories

func (m *memoryPartners) Create(_ context.Context, p *domain.Partner) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextPartnerID++
	p.ID = m.nextPartnerID
	p.CreatedAt = time.Now().UTC()
	m.partners[p.ID] = *p
	return nil
}

func (m *memoryPartners) GetByID(_ context.Context, id int64) (*domain.Partner, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.partners[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &p, nil
}

func (m *memoryPartners) GetByAPIKey(_ context.Context, apiKey string) (*domain.Partner, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.partners {
		if p.APIKey == apiKey {
			cp := p
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (m *memoryPartners) List(_ context.Context, page, pageSize int) ([]domain.Partner, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]domain.Partner, 0, len(m.partners))
	for _, p := range m.partners {
		all = append(all, p)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	return paginate(all, page, pageSize), len(all), nil
}

func (m *memoryPartners) Update(_ context.Context, p *domain.Partner) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.partners[p.ID]; !ok {
		return ErrNotFound
	}
	m.partners[p.ID] = *p
	return nil
}

func (m *memoryPartners) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.partners[id]; !ok {
		return ErrNotFound
	}
	delete(m.partners, id)
	return nil
}

type memoryRoutes MemoryRepositories

func (m *memoryRoutes) Create(_ context.Context, r *domain.Route) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextRouteID++
	r.ID = m.nextRouteID
	r.CreatedAt = time.Now().UTC()
	m.routes[r.ID] = *r
	return nil
}

func (m *memoryRoutes) GetByID(_ context.Context, id int64) (*domain.Route, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.routes[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &r, nil
}

func (m *memoryRoutes) List(_ context.Context, page, pageSize int) ([]domain.Route, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]domain.Route, 0, len(m.routes))
	for _, r := range m.routes {
		all = append(all, r)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	return paginate(all, page, pageSize), len(all), nil
}

func (m *memoryRoutes) ListActive(_ context.Context) ([]domain.Route, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]domain.Route, 0)
	for _, r := range m.routes {
		if r.Active {
			out = append(out, r)
		}
	}
	// Ordena do prefixo mais longo para o mais curto: a primeira rota que
	// casar com o path da requisicao vence, entao a mais especifica precisa
	// vir antes das mais genericas.
	sort.Slice(out, func(i, j int) bool { return len(out[i].PathPrefix) > len(out[j].PathPrefix) })
	return out, nil
}

func (m *memoryRoutes) Update(_ context.Context, r *domain.Route) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.routes[r.ID]; !ok {
		return ErrNotFound
	}
	m.routes[r.ID] = *r
	return nil
}

func (m *memoryRoutes) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.routes[id]; !ok {
		return ErrNotFound
	}
	delete(m.routes, id)
	return nil
}

type memoryPolicies MemoryRepositories

func (m *memoryPolicies) Create(_ context.Context, pol *domain.RateLimitPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextPolicyID++
	pol.ID = m.nextPolicyID
	pol.CreatedAt = time.Now().UTC()
	m.policies[pol.ID] = *pol
	return nil
}

func (m *memoryPolicies) List(_ context.Context, page, pageSize int) ([]domain.RateLimitPolicy, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]domain.RateLimitPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		all = append(all, p)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	return paginate(all, page, pageSize), len(all), nil
}

func (m *memoryPolicies) FindEffective(_ context.Context, partnerID, routeID int64) (*domain.RateLimitPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var fallback *domain.RateLimitPolicy
	for _, p := range m.policies {
		if p.PartnerID != partnerID {
			continue
		}
		if p.RouteID == routeID {
			cp := p
			return &cp, nil
		}
		if p.RouteID == 0 {
			cp := p
			fallback = &cp
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	return nil, ErrNotFound
}

func (m *memoryPolicies) Delete(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.policies[id]; !ok {
		return ErrNotFound
	}
	delete(m.policies, id)
	return nil
}

type memoryAudit MemoryRepositories

func (m *memoryAudit) Insert(_ context.Context, log *domain.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextAuditID++
	log.ID = m.nextAuditID
	log.CreatedAt = time.Now().UTC()
	m.audit = append(m.audit, *log)
	return nil
}

func (m *memoryAudit) List(_ context.Context, page, pageSize int) ([]domain.AuditLog, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	all := make([]domain.AuditLog, len(m.audit))
	copy(all, m.audit)
	sort.Slice(all, func(i, j int) bool { return all[i].ID > all[j].ID })
	return paginate(all, page, pageSize), len(all), nil
}

func (m *memoryAudit) TopConsumers(_ context.Context, since time.Time, limit int) ([]ConsumerStat, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	type key struct {
		partnerID int64
		routeID   int64
	}
	counts := make(map[key]int64)
	for _, a := range m.audit {
		if a.CreatedAt.Before(since) {
			continue
		}
		counts[key{a.PartnerID, a.RouteID}]++
	}
	out := make([]ConsumerStat, 0, len(counts))
	for k, c := range counts {
		partnerName := ""
		if p, ok := m.partners[k.partnerID]; ok {
			partnerName = p.Name
		}
		routePath := ""
		if r, ok := m.routes[k.routeID]; ok {
			routePath = r.PathPrefix
		}
		out = append(out, ConsumerStat{
			PartnerID:   k.partnerID,
			PartnerName: partnerName,
			RouteID:     k.routeID,
			RoutePath:   routePath,
			Requests:    c,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Requests > out[j].Requests })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func paginate[T any](all []T, page, pageSize int) []T {
	page, pageSize = domain.NormalizePageParams(page, pageSize)
	start := (page - 1) * pageSize
	if start >= len(all) {
		return []T{}
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}
	return all[start:end]
}
