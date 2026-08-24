package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/hagile/comporta/internal/domain"
	"github.com/hagile/comporta/internal/metrics"
	"github.com/hagile/comporta/internal/proxy"
	"github.com/hagile/comporta/internal/ratelimit"
	"github.com/hagile/comporta/internal/stats"
	"github.com/hagile/comporta/internal/store"
)

// apiKeyHeader e o cabecalho que os parceiros usam para se autenticar no
// gateway. Optamos por um header dedicado em vez de Authorization: Bearer
// porque a chave aqui identifica o parceiro (nao um usuario final) e alguns
// dos sistemas legados que ele integra ja usam Authorization para outra coisa.
const apiKeyHeader = "X-API-Key"

// GatewayHandler e o coracao do produto: autentica pela api key, verifica
// cota no rate limiter, loga a requisicao para auditoria e faz o proxy para
// o upstream configurado.
type GatewayHandler struct {
	Router     *proxy.Router
	Limiter    *ratelimit.Limiter
	Partners   store.PartnerRepository
	Policies   store.PolicyRepository
	Audit      store.AuditRepository
	Aggregator *stats.Aggregator
	Log        *zap.Logger
}

func (h *GatewayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	route, ok := h.Router.Match(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "nenhuma rota configurada para este caminho")
		return
	}

	apiKey := r.Header.Get(apiKeyHeader)
	if apiKey == "" {
		metrics.AuthFailures.WithLabelValues("ausente").Inc()
		writeError(w, http.StatusUnauthorized, "informe a api key no cabecalho "+apiKeyHeader)
		return
	}

	partner, err := h.Partners.GetByAPIKey(r.Context(), apiKey)
	if err != nil || !partner.Active {
		metrics.AuthFailures.WithLabelValues("invalida").Inc()
		writeError(w, http.StatusUnauthorized, "api key invalida ou parceiro inativo")
		return
	}

	policy, err := h.Policies.FindEffective(r.Context(), partner.ID, route.ID)
	if err != nil {
		// Sem politica de rate limit cadastrada o parceiro nao tem permissao
		// de acesso a rota: acesso precisa ser concedido explicitamente, nao
		// e o comportamento padrao do gateway.
		writeError(w, http.StatusForbidden, "parceiro sem permissao configurada para esta rota")
		return
	}

	limitKey := fmt.Sprintf("ratelimit:%d:%d", partner.ID, route.ID)
	result, err := h.Limiter.Allow(r.Context(), limitKey, policy.RequestsPerMinute, time.Minute)
	if err != nil {
		h.Log.Error("falha ao consultar rate limiter", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "falha interna ao verificar cota")
		return
	}
	if !result.Allowed {
		metrics.RateLimitRejections.WithLabelValues(partner.Name, route.PathPrefix).Inc()
		retrySeconds := int(result.RetryAfter.Seconds()) + 1
		w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(policy.RequestsPerMinute))
		writeError(w, http.StatusTooManyRequests, "cota excedida, tente novamente em instantes")
		h.recordAudit(*partner, route, r, http.StatusTooManyRequests, time.Since(start))
		return
	}

	recorder := newStatusRecorder(w)
	h.Router.ServeHTTP(route, recorder, r)

	elapsed := time.Since(start)
	metrics.RequestsTotal.WithLabelValues(partner.Name, route.PathPrefix, strconv.Itoa(recorder.status)).Inc()
	metrics.RequestDuration.WithLabelValues(route.PathPrefix).Observe(elapsed.Seconds())
	if recorder.status >= 500 {
		metrics.UpstreamErrors.WithLabelValues(route.PathPrefix).Inc()
	}
	h.Aggregator.Record(partner.ID, partner.Name, route.ID, route.PathPrefix, recorder.status)
	h.recordAudit(*partner, route, r, recorder.status, elapsed)
}

// recordAudit grava o log de auditoria em segundo plano: a auditoria e
// importante, mas nao deve adicionar a latencia do Oracle na resposta ao
// parceiro que esta apenas consumindo o upstream.
func (h *GatewayHandler) recordAudit(partner domain.Partner, route domain.Route, r *http.Request, status int, elapsed time.Duration) {
	log := domain.AuditLog{
		PartnerID:  partner.ID,
		RouteID:    route.ID,
		Method:     r.Method,
		Path:       r.URL.Path,
		StatusCode: status,
		LatencyMs:  elapsed.Milliseconds(),
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := h.Audit.Insert(ctx, &log); err != nil {
			h.Log.Error("falha ao gravar log de auditoria", zap.Error(err))
		}
	}()
}
