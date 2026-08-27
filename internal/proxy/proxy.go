// Package proxy resolve qual rota configurada atende um path de entrada e
// encaminha a requisicao para o servico upstream correspondente.
package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/walberst/comporta/internal/domain"
)

// Router mantem a tabela de rotas ativas em memoria, atualizada
// periodicamente a partir do Oracle. Consultar o banco a cada requisicao de
// proxy adicionaria uma latencia desnecessaria no caminho quente.
type Router struct {
	mu      sync.RWMutex
	routes  []domain.Route // sempre ordenadas do prefixo mais longo para o mais curto
	proxies map[int64]*httputil.ReverseProxy
}

func NewRouter() *Router {
	return &Router{proxies: make(map[int64]*httputil.ReverseProxy)}
}

// SetRoutes substitui a tabela de rotas atomically. Chamado pelo loop de
// refresh do gateway.
func (rt *Router) SetRoutes(routes []domain.Route) {
	sorted := make([]domain.Route, len(routes))
	copy(sorted, routes)
	// Prefixo mais longo primeiro: "/billing/v2" precisa vencer "/billing"
	// quando ambos estao cadastrados.
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && len(sorted[j].PathPrefix) > len(sorted[j-1].PathPrefix); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	proxies := make(map[int64]*httputil.ReverseProxy, len(sorted))
	for _, r := range sorted {
		if p, err := newReverseProxy(r); err == nil {
			proxies[r.ID] = p
		}
	}

	rt.mu.Lock()
	rt.routes = sorted
	rt.proxies = proxies
	rt.mu.Unlock()
}

// Match retorna a rota ativa cujo prefixo casa com o path informado.
func (rt *Router) Match(path string) (domain.Route, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	for _, r := range rt.routes {
		if strings.HasPrefix(path, r.PathPrefix) {
			return r, true
		}
	}
	return domain.Route{}, false
}

// ServeHTTP encaminha a requisicao para o upstream da rota. O status code
// real da resposta upstream e capturado via responseRecorder para poder ser
// usado no log de auditoria pelo middleware que chama este metodo.
func (rt *Router) ServeHTTP(route domain.Route, w http.ResponseWriter, r *http.Request) {
	rt.mu.RLock()
	p, ok := rt.proxies[route.ID]
	rt.mu.RUnlock()
	if !ok {
		http.Error(w, "upstream indisponivel para esta rota", http.StatusBadGateway)
		return
	}
	p.ServeHTTP(w, r)
}

func newReverseProxy(route domain.Route) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(route.UpstreamURL)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	prefix := route.PathPrefix
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		if route.StripPrefix {
			trimmed := strings.TrimPrefix(req.URL.Path, prefix)
			if trimmed == "" {
				trimmed = "/"
			}
			if !strings.HasPrefix(trimmed, "/") {
				trimmed = "/" + trimmed
			}
			req.URL.Path = trimmed
		}
		// Identifica para o upstream que a chamada veio pelo gateway, util
		// para o upstream nao precisar reimplementar checagem de origem.
		req.Header.Set("X-Forwarded-By", "comporta-gateway")
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "erro ao contatar servico upstream", http.StatusBadGateway)
	}
	return proxy, nil
}
