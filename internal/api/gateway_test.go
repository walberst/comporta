package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/walberst/comporta/internal/api"
	"github.com/walberst/comporta/internal/domain"
	"github.com/walberst/comporta/internal/proxy"
	"github.com/walberst/comporta/internal/ratelimit"
	"github.com/walberst/comporta/internal/stats"
	"github.com/walberst/comporta/internal/store"
)

type testEnv struct {
	handler  *api.GatewayHandler
	repos    store.Repositories
	upstream *httptest.Server
	partner  domain.Partner
	route    domain.Route
}

func newTestEnv(t *testing.T, requestsPerMinute int) *testEnv {
	t.Helper()

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("upstream ok"))
	}))
	t.Cleanup(upstream.Close)

	mem := store.NewMemoryRepositories()
	repos := mem.AsRepositories()
	ctx := context.Background()

	partner := domain.Partner{Name: "Aurora Fintech", APIKey: "cpk_test_key", Active: true}
	require.NoError(t, repos.Partners.Create(ctx, &partner))

	route := domain.Route{PathPrefix: "/billing", UpstreamURL: upstream.URL, StripPrefix: true, Active: true}
	require.NoError(t, repos.Routes.Create(ctx, &route))

	policy := domain.RateLimitPolicy{PartnerID: partner.ID, RouteID: route.ID, RequestsPerMinute: requestsPerMinute}
	require.NoError(t, repos.Policies.Create(ctx, &policy))

	router := proxy.NewRouter()
	router.SetRoutes([]domain.Route{route})

	handler := &api.GatewayHandler{
		Router:     router,
		Limiter:    ratelimit.NewLimiter(rdb),
		Partners:   repos.Partners,
		Policies:   repos.Policies,
		Audit:      repos.Audit,
		Aggregator: stats.NewAggregator(),
		Log:        zap.NewNop(),
	}

	return &testEnv{handler: handler, repos: repos, upstream: upstream, partner: partner, route: route}
}

func TestGatewayHandler_RequisicaoSemAPIKey(t *testing.T) {
	env := newTestEnv(t, 10)
	req := httptest.NewRequest(http.MethodGet, "/billing/invoices", nil)
	rec := httptest.NewRecorder()

	env.handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGatewayHandler_APIKeyInvalida(t *testing.T) {
	env := newTestEnv(t, 10)
	req := httptest.NewRequest(http.MethodGet, "/billing/invoices", nil)
	req.Header.Set("X-API-Key", "chave-que-nao-existe")
	rec := httptest.NewRecorder()

	env.handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGatewayHandler_RotaNaoConfigurada(t *testing.T) {
	env := newTestEnv(t, 10)
	req := httptest.NewRequest(http.MethodGet, "/nao-existe", nil)
	req.Header.Set("X-API-Key", env.partner.APIKey)
	rec := httptest.NewRecorder()

	env.handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGatewayHandler_ParceiroSemPoliticaParaRota(t *testing.T) {
	env := newTestEnv(t, 10)
	ctx := context.Background()

	outroParceiro := domain.Partner{Name: "Sem Permissao", APIKey: "cpk_outro", Active: true}
	require.NoError(t, env.repos.Partners.Create(ctx, &outroParceiro))

	req := httptest.NewRequest(http.MethodGet, "/billing/invoices", nil)
	req.Header.Set("X-API-Key", outroParceiro.APIKey)
	rec := httptest.NewRecorder()

	env.handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestGatewayHandler_ProxyBemSucedidoGravaAuditoria(t *testing.T) {
	env := newTestEnv(t, 10)
	req := httptest.NewRequest(http.MethodGet, "/billing/invoices", nil)
	req.Header.Set("X-API-Key", env.partner.APIKey)
	rec := httptest.NewRecorder()

	env.handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "upstream ok", rec.Body.String())

	require.Eventually(t, func() bool {
		logs, total, err := env.repos.Audit.List(context.Background(), 1, 10)
		return err == nil && total == 1 && logs[0].StatusCode == http.StatusOK
	}, time.Second, 10*time.Millisecond, "o log de auditoria deveria ser gravado apos a resposta")
}

func TestGatewayHandler_EstouroDeCotaRetorna429ComRetryAfter(t *testing.T) {
	env := newTestEnv(t, 1)

	newReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/billing/invoices", nil)
		req.Header.Set("X-API-Key", env.partner.APIKey)
		return req
	}

	rec1 := httptest.NewRecorder()
	env.handler.ServeHTTP(rec1, newReq())
	require.Equal(t, http.StatusOK, rec1.Code)

	rec2 := httptest.NewRecorder()
	env.handler.ServeHTTP(rec2, newReq())
	require.Equal(t, http.StatusTooManyRequests, rec2.Code)
	require.NotEmpty(t, rec2.Header().Get("Retry-After"))
}
