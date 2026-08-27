// Comando gateway sobe o servidor HTTP principal do Comporta: proxy
// autenticado com rate limiting, endpoints administrativos, metricas
// Prometheus e o websocket de status ao vivo.
package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/walberst/comporta/internal/api"
	"github.com/walberst/comporta/internal/config"
	"github.com/walberst/comporta/internal/logging"
	"github.com/walberst/comporta/internal/proxy"
	"github.com/walberst/comporta/internal/ratelimit"
	"github.com/walberst/comporta/internal/stats"
	"github.com/walberst/comporta/internal/store"
	"github.com/walberst/comporta/internal/wshub"
)

// routeRefreshInterval controla a defasagem maxima entre uma mudanca de rota
// no cadastro e ela valer no caminho quente do proxy. Recarregar a cada
// requisicao seria simples, mas colocaria o Oracle no meio de toda chamada
// de parceiro; um cache com refresh periodico e o compromisso padrao para
// gateways deste tipo.
const routeRefreshInterval = 5 * time.Second

func main() {
	cfg := config.Load()

	log, err := logging.New(cfg.LogLevel)
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()

	repos, closeStore, err := buildRepositories(cfg, log)
	if err != nil {
		log.Fatal("falha ao inicializar armazenamento", zap.Error(err))
	}
	defer closeStore()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer func() { _ = rdb.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal("falha ao conectar no redis", zap.Error(err))
	}

	router := proxy.NewRouter()
	if err := refreshRoutes(ctx, repos.Routes, router); err != nil {
		log.Warn("falha ao carregar rotas iniciais", zap.Error(err))
	}
	go routeRefreshLoop(ctx, repos.Routes, router, log)

	limiter := ratelimit.NewLimiter(rdb)
	aggregator := stats.NewAggregator()
	hub := wshub.NewHub(log)
	go broadcastLoop(ctx, aggregator, hub, cfg.WSBroadcastInterval)

	adminHandler := &api.AdminHandler{Repos: repos, Log: log}
	gatewayHandler := &api.GatewayHandler{
		Router:     router,
		Limiter:    limiter,
		Partners:   repos.Partners,
		Policies:   repos.Policies,
		Audit:      repos.Audit,
		Aggregator: aggregator,
		Log:        log,
	}

	handler := api.NewRouter(adminHandler, gatewayHandler, hub, cfg.AdminToken, log)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("gateway ouvindo", zap.String("porta", cfg.HTTPPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("falha ao subir servidor HTTP", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("encerrando gateway")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("erro ao encerrar servidor HTTP", zap.Error(err))
	}
}

func buildRepositories(cfg config.Config, log *zap.Logger) (store.Repositories, func(), error) {
	if cfg.UseMemoryStore || cfg.OracleDSN == "" {
		log.Warn("subindo com armazenamento em memoria: dados nao sao persistidos, use apenas para desenvolvimento local")
		mem := store.NewMemoryRepositories()
		return mem.AsRepositories(), func() {}, nil
	}

	db, err := store.OpenOracle(cfg.OracleDSN)
	if err != nil {
		return store.Repositories{}, nil, err
	}
	oracleRepos := store.NewOracleRepositories(db)
	return oracleRepos.AsRepositories(), func() { _ = db.Close() }, nil
}

func refreshRoutes(ctx context.Context, repo store.RouteRepository, router *proxy.Router) error {
	routes, err := repo.ListActive(ctx)
	if err != nil {
		return err
	}
	router.SetRoutes(routes)
	return nil
}

func routeRefreshLoop(ctx context.Context, repo store.RouteRepository, router *proxy.Router, log *zap.Logger) {
	ticker := time.NewTicker(routeRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := refreshRoutes(ctx, repo, router); err != nil {
				log.Warn("falha ao atualizar tabela de rotas", zap.Error(err))
			}
		}
	}
}

func broadcastLoop(ctx context.Context, aggregator *stats.Aggregator, hub *wshub.Hub, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hub.Broadcast(aggregator.Snapshot(10))
		}
	}
}
