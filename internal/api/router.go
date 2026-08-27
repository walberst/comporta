package api

import (
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/walberst/comporta/internal/wshub"
)

// NewRouter monta o multiplexador HTTP completo do gateway: endpoints
// administrativos (protegidos por token), observabilidade, websocket de
// metricas e, por fim, o proxy propriamente dito como rota curinga.
func NewRouter(admin *AdminHandler, gateway *GatewayHandler, hub *wshub.Hub, adminToken string, log *zap.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /ws", hub.ServeWS)

	protect := func(h http.HandlerFunc) http.Handler {
		return requireAdminToken(adminToken, h)
	}

	mux.Handle("POST /admin/partners", protect(admin.CreatePartner))
	mux.Handle("GET /admin/partners", protect(admin.ListPartners))
	mux.Handle("GET /admin/partners/{id}", protect(admin.GetPartner))
	mux.Handle("PUT /admin/partners/{id}", protect(admin.UpdatePartner))
	mux.Handle("DELETE /admin/partners/{id}", protect(admin.DeletePartner))

	mux.Handle("POST /admin/routes", protect(admin.CreateRoute))
	mux.Handle("GET /admin/routes", protect(admin.ListRoutes))
	mux.Handle("GET /admin/routes/{id}", protect(admin.GetRoute))
	mux.Handle("PUT /admin/routes/{id}", protect(admin.UpdateRoute))
	mux.Handle("DELETE /admin/routes/{id}", protect(admin.DeleteRoute))

	mux.Handle("POST /admin/policies", protect(admin.CreatePolicy))
	mux.Handle("GET /admin/policies", protect(admin.ListPolicies))
	mux.Handle("DELETE /admin/policies/{id}", protect(admin.DeletePolicy))

	mux.Handle("GET /admin/audit-logs", protect(admin.ListAuditLogs))

	// Qualquer caminho que nao case com as rotas acima e trafego de parceiro
	// e vai para o motor de proxy.
	mux.Handle("/", gateway)

	return withCORS(mux)
}

// requireAdminToken protege os endpoints administrativos com um token
// simples (Bearer). Um esquema de autenticacao mais rico (OAuth2, mTLS)
// faria sentido num gateway real de producao, mas foge do escopo aqui: o
// que se quer demonstrar e a separacao entre trafego administrativo e
// trafego de parceiro, cada um com sua propria autenticacao.
func requireAdminToken(token string, next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		expected := "Bearer " + token
		if header == "" || header != expected {
			writeError(w, http.StatusUnauthorized, "token administrativo invalido ou ausente")
			return
		}
		next(w, r)
	})
}

// withCORS libera o painel Nuxt (rodando em outra origem durante o
// desenvolvimento) a chamar os endpoints administrativos e o websocket sem
// exigir um proxy reverso extra so para isso.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, "+apiKeyHeader)
		if strings.EqualFold(r.Method, http.MethodOptions) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
