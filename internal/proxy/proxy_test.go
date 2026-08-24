package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hagile/comporta/internal/domain"
	"github.com/hagile/comporta/internal/proxy"
)

func TestRouter_EncaminhaParaUpstreamCorretoEStripaPrefixo(t *testing.T) {
	var receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	router := proxy.NewRouter()
	router.SetRoutes([]domain.Route{
		{ID: 1, PathPrefix: "/billing", UpstreamURL: upstream.URL, StripPrefix: true, Active: true},
	})

	route, ok := router.Match("/billing/invoices")
	require.True(t, ok)

	req := httptest.NewRequest(http.MethodGet, "/billing/invoices", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(route, rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "/invoices", receivedPath)
}

func TestRouter_MantemPrefixoQuandoStripDesligado(t *testing.T) {
	var receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	router := proxy.NewRouter()
	router.SetRoutes([]domain.Route{
		{ID: 1, PathPrefix: "/billing", UpstreamURL: upstream.URL, StripPrefix: false, Active: true},
	})

	route, _ := router.Match("/billing/invoices")
	req := httptest.NewRequest(http.MethodGet, "/billing/invoices", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(route, rec, req)

	require.Equal(t, "/billing/invoices", receivedPath)
}

func TestRouter_PrefixoMaisEspecificoVence(t *testing.T) {
	var receivedByGeneric, receivedBySpecific bool

	generic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedByGeneric = true
	}))
	defer generic.Close()

	specific := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBySpecific = true
	}))
	defer specific.Close()

	router := proxy.NewRouter()
	router.SetRoutes([]domain.Route{
		{ID: 1, PathPrefix: "/billing", UpstreamURL: generic.URL, Active: true},
		{ID: 2, PathPrefix: "/billing/v2", UpstreamURL: specific.URL, Active: true},
	})

	route, ok := router.Match("/billing/v2/invoices")
	require.True(t, ok)
	require.Equal(t, int64(2), route.ID)

	req := httptest.NewRequest(http.MethodGet, "/billing/v2/invoices", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(route, rec, req)

	require.True(t, receivedBySpecific)
	require.False(t, receivedByGeneric)
}

func TestRouter_SemRotaCorrespondente(t *testing.T) {
	router := proxy.NewRouter()
	router.SetRoutes([]domain.Route{
		{ID: 1, PathPrefix: "/billing", UpstreamURL: "http://example.invalid", Active: true},
	})

	_, ok := router.Match("/inventory/stock")
	require.False(t, ok)
}

func TestRouter_ErroDeUpstreamRetorna502(t *testing.T) {
	router := proxy.NewRouter()
	// Porta 0 no host garante uma falha de conexao imediata e determinista.
	router.SetRoutes([]domain.Route{
		{ID: 1, PathPrefix: "/billing", UpstreamURL: "http://127.0.0.1:0", Active: true},
	})

	route, ok := router.Match("/billing/invoices")
	require.True(t, ok)

	req := httptest.NewRequest(http.MethodGet, "/billing/invoices", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(route, rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	require.NotEmpty(t, body)
}
