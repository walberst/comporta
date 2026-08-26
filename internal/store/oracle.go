package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/hagile/comporta/internal/domain"
	// driver go-ora: implementacao pura em Go do protocolo Oracle (TNS), sem
	// precisar do Oracle Instant Client instalado na imagem, o que mantem o
	// Dockerfile final leve.
	_ "github.com/sijms/go-ora/v2"
)

// OpenOracle abre o pool de conexoes com o Oracle usando o driver go-ora. O
// DSN esperado segue o formato oracle://usuario:senha@host:porta/servico.
func OpenOracle(dsn string) (*sql.DB, error) {
	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return nil, fmt.Errorf("abrindo conexao oracle: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}

// OracleRepositories implementa os repositorios contra um Oracle real.
type OracleRepositories struct {
	db *sql.DB
}

func NewOracleRepositories(db *sql.DB) *OracleRepositories {
	return &OracleRepositories{db: db}
}

func (o *OracleRepositories) AsRepositories() Repositories {
	return Repositories{
		Partners: (*oraclePartners)(o),
		Routes:   (*oracleRoutes)(o),
		Policies: (*oraclePolicies)(o),
		Audit:    (*oracleAudit)(o),
	}
}

func boolToNum(b bool) int {
	if b {
		return 1
	}
	return 0
}

type oraclePartners OracleRepositories

func (r *oraclePartners) Create(ctx context.Context, p *domain.Partner) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO partners (name, api_key, active) VALUES (:1, :2, :3)`,
		p.Name, p.APIKey, boolToNum(p.Active))
	if err != nil {
		return fmt.Errorf("inserindo partner: %w", err)
	}
	// A api_key e unica por regra de negocio, entao ela e uma chave segura
	// para recuperar o id gerado pela IDENTITY column sem depender de
	// RETURNING INTO (mais simples de portar entre drivers Oracle).
	row := r.db.QueryRowContext(ctx, `SELECT id, created_at FROM partners WHERE api_key = :1`, p.APIKey)
	return row.Scan(&p.ID, &p.CreatedAt)
}

func (r *oraclePartners) GetByID(ctx context.Context, id int64) (*domain.Partner, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, api_key, active, created_at FROM partners WHERE id = :1`, id)
	return scanPartner(row)
}

func (r *oraclePartners) GetByAPIKey(ctx context.Context, apiKey string) (*domain.Partner, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, api_key, active, created_at FROM partners WHERE api_key = :1`, apiKey)
	return scanPartner(row)
}

func scanPartner(row *sql.Row) (*domain.Partner, error) {
	var p domain.Partner
	var active int
	if err := row.Scan(&p.ID, &p.Name, &p.APIKey, &active, &p.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.Active = active == 1
	return &p, nil
}

func (r *oraclePartners) List(ctx context.Context, page, pageSize int) ([]domain.Partner, int, error) {
	page, pageSize = domain.NormalizePageParams(page, pageSize)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM partners`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("contando partners: %w", err)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, api_key, active, created_at FROM partners
		 ORDER BY id OFFSET :1 ROWS FETCH NEXT :2 ROWS ONLY`,
		(page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("listando partners: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.Partner, 0, pageSize)
	for rows.Next() {
		var p domain.Partner
		var active int
		if err := rows.Scan(&p.ID, &p.Name, &p.APIKey, &active, &p.CreatedAt); err != nil {
			return nil, 0, err
		}
		p.Active = active == 1
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func (r *oraclePartners) Update(ctx context.Context, p *domain.Partner) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE partners SET name = :1, active = :2 WHERE id = :3`,
		p.Name, boolToNum(p.Active), p.ID)
	if err != nil {
		return fmt.Errorf("atualizando partner: %w", err)
	}
	return checkAffected(res)
}

func (r *oraclePartners) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM partners WHERE id = :1`, id)
	if err != nil {
		return fmt.Errorf("removendo partner: %w", err)
	}
	return checkAffected(res)
}

type oracleRoutes OracleRepositories

func (r *oracleRoutes) Create(ctx context.Context, rt *domain.Route) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO routes (path_prefix, upstream_url, strip_prefix, active) VALUES (:1, :2, :3, :4)`,
		rt.PathPrefix, rt.UpstreamURL, boolToNum(rt.StripPrefix), boolToNum(rt.Active))
	if err != nil {
		return fmt.Errorf("inserindo route: %w", err)
	}
	row := r.db.QueryRowContext(ctx, `SELECT id, created_at FROM routes WHERE path_prefix = :1`, rt.PathPrefix)
	return row.Scan(&rt.ID, &rt.CreatedAt)
}

func (r *oracleRoutes) GetByID(ctx context.Context, id int64) (*domain.Route, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, path_prefix, upstream_url, strip_prefix, active, created_at FROM routes WHERE id = :1`, id)
	return scanRoute(row)
}

func scanRoute(row *sql.Row) (*domain.Route, error) {
	var rt domain.Route
	var strip, active int
	if err := row.Scan(&rt.ID, &rt.PathPrefix, &rt.UpstreamURL, &strip, &active, &rt.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rt.StripPrefix = strip == 1
	rt.Active = active == 1
	return &rt, nil
}

func (r *oracleRoutes) List(ctx context.Context, page, pageSize int) ([]domain.Route, int, error) {
	page, pageSize = domain.NormalizePageParams(page, pageSize)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM routes`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("contando routes: %w", err)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, path_prefix, upstream_url, strip_prefix, active, created_at FROM routes
		 ORDER BY id OFFSET :1 ROWS FETCH NEXT :2 ROWS ONLY`,
		(page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("listando routes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.Route, 0, pageSize)
	for rows.Next() {
		var rt domain.Route
		var strip, active int
		if err := rows.Scan(&rt.ID, &rt.PathPrefix, &rt.UpstreamURL, &strip, &active, &rt.CreatedAt); err != nil {
			return nil, 0, err
		}
		rt.StripPrefix = strip == 1
		rt.Active = active == 1
		out = append(out, rt)
	}
	return out, total, rows.Err()
}

func (r *oracleRoutes) ListActive(ctx context.Context) ([]domain.Route, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, path_prefix, upstream_url, strip_prefix, active, created_at FROM routes
		 WHERE active = 1 ORDER BY LENGTH(path_prefix) DESC`)
	if err != nil {
		return nil, fmt.Errorf("listando routes ativas: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.Route, 0)
	for rows.Next() {
		var rt domain.Route
		var strip, active int
		if err := rows.Scan(&rt.ID, &rt.PathPrefix, &rt.UpstreamURL, &strip, &active, &rt.CreatedAt); err != nil {
			return nil, err
		}
		rt.StripPrefix = strip == 1
		rt.Active = active == 1
		out = append(out, rt)
	}
	return out, rows.Err()
}

func (r *oracleRoutes) Update(ctx context.Context, rt *domain.Route) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE routes SET path_prefix = :1, upstream_url = :2, strip_prefix = :3, active = :4 WHERE id = :5`,
		rt.PathPrefix, rt.UpstreamURL, boolToNum(rt.StripPrefix), boolToNum(rt.Active), rt.ID)
	if err != nil {
		return fmt.Errorf("atualizando route: %w", err)
	}
	return checkAffected(res)
}

func (r *oracleRoutes) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM routes WHERE id = :1`, id)
	if err != nil {
		return fmt.Errorf("removendo route: %w", err)
	}
	return checkAffected(res)
}

type oraclePolicies OracleRepositories

func (r *oraclePolicies) Create(ctx context.Context, pol *domain.RateLimitPolicy) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO rate_limit_policies (partner_id, route_id, requests_per_minute) VALUES (:1, :2, :3)`,
		pol.PartnerID, pol.RouteID, pol.RequestsPerMinute)
	if err != nil {
		return fmt.Errorf("inserindo policy: %w", err)
	}
	row := r.db.QueryRowContext(ctx,
		`SELECT id, created_at FROM rate_limit_policies WHERE partner_id = :1 AND route_id = :2`,
		pol.PartnerID, pol.RouteID)
	return row.Scan(&pol.ID, &pol.CreatedAt)
}

func (r *oraclePolicies) List(ctx context.Context, page, pageSize int) ([]domain.RateLimitPolicy, int, error) {
	page, pageSize = domain.NormalizePageParams(page, pageSize)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM rate_limit_policies`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("contando policies: %w", err)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, partner_id, route_id, requests_per_minute, created_at FROM rate_limit_policies
		 ORDER BY id OFFSET :1 ROWS FETCH NEXT :2 ROWS ONLY`,
		(page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("listando policies: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.RateLimitPolicy, 0, pageSize)
	for rows.Next() {
		var p domain.RateLimitPolicy
		if err := rows.Scan(&p.ID, &p.PartnerID, &p.RouteID, &p.RequestsPerMinute, &p.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func (r *oraclePolicies) FindEffective(ctx context.Context, partnerID, routeID int64) (*domain.RateLimitPolicy, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, partner_id, route_id, requests_per_minute, created_at FROM rate_limit_policies
		 WHERE partner_id = :1 AND route_id IN (:2, 0)
		 ORDER BY CASE WHEN route_id = :2 THEN 0 ELSE 1 END
		 FETCH FIRST 1 ROW ONLY`,
		partnerID, routeID)
	var p domain.RateLimitPolicy
	if err := row.Scan(&p.ID, &p.PartnerID, &p.RouteID, &p.RequestsPerMinute, &p.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *oraclePolicies) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM rate_limit_policies WHERE id = :1`, id)
	if err != nil {
		return fmt.Errorf("removendo policy: %w", err)
	}
	return checkAffected(res)
}

type oracleAudit OracleRepositories

func (r *oracleAudit) Insert(ctx context.Context, log *domain.AuditLog) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_logs (partner_id, route_id, method, path, status_code, latency_ms)
		 VALUES (:1, :2, :3, :4, :5, :6)`,
		log.PartnerID, log.RouteID, log.Method, log.Path, log.StatusCode, log.LatencyMs)
	if err != nil {
		return fmt.Errorf("inserindo audit log: %w", err)
	}
	return nil
}

func (r *oracleAudit) List(ctx context.Context, page, pageSize int) ([]domain.AuditLog, int, error) {
	page, pageSize = domain.NormalizePageParams(page, pageSize)
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("contando audit logs: %w", err)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, partner_id, route_id, method, path, status_code, latency_ms, created_at FROM audit_logs
		 ORDER BY id DESC OFFSET :1 ROWS FETCH NEXT :2 ROWS ONLY`,
		(page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("listando audit logs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]domain.AuditLog, 0, pageSize)
	for rows.Next() {
		var a domain.AuditLog
		if err := rows.Scan(&a.ID, &a.PartnerID, &a.RouteID, &a.Method, &a.Path, &a.StatusCode, &a.LatencyMs, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func (r *oracleAudit) TopConsumers(ctx context.Context, since time.Time, limit int) ([]ConsumerStat, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT a.partner_id, p.name, a.route_id, rt.path_prefix, COUNT(*) AS requests
		 FROM audit_logs a
		 JOIN partners p ON p.id = a.partner_id
		 JOIN routes rt ON rt.id = a.route_id
		 WHERE a.created_at >= :1
		 GROUP BY a.partner_id, p.name, a.route_id, rt.path_prefix
		 ORDER BY requests DESC
		 FETCH FIRST :2 ROWS ONLY`,
		since, limit)
	if err != nil {
		return nil, fmt.Errorf("consultando top consumidores: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]ConsumerStat, 0, limit)
	for rows.Next() {
		var c ConsumerStat
		if err := rows.Scan(&c.PartnerID, &c.PartnerName, &c.RouteID, &c.RoutePath, &c.Requests); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func checkAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
