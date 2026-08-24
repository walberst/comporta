// Package metrics expoe os contadores e histogramas Prometheus do gateway.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "comporta_requests_total",
		Help: "Total de requisicoes recebidas pelo gateway, por parceiro, rota e status.",
	}, []string{"partner", "route", "status"})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "comporta_request_duration_seconds",
		Help:    "Duracao das requisicoes proxied pelo gateway.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route"})

	RateLimitRejections = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "comporta_rate_limit_rejections_total",
		Help: "Total de requisicoes rejeitadas por estouro de cota, por parceiro e rota.",
	}, []string{"partner", "route"})

	AuthFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "comporta_auth_failures_total",
		Help: "Total de requisicoes rejeitadas por falha de autenticacao (api key invalida ou ausente).",
	}, []string{"reason"})

	UpstreamErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "comporta_upstream_errors_total",
		Help: "Total de erros ao contatar servicos upstream, por rota.",
	}, []string{"route"})

	WSClientsConnected = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "comporta_ws_clients_connected",
		Help: "Numero de clientes conectados ao websocket de metricas ao vivo.",
	})
)
