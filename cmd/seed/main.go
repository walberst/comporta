// Comando seed popula o Oracle com parceiros, rotas e politicas de exemplo.
// E sempre um passo manual e separado (go run ./cmd/seed): o docker compose
// up sobe o banco limpo por padrao, para quem for usar o gateway de verdade
// nao herdar dados fake sem querer.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hagile/comporta/internal/config"
	"github.com/hagile/comporta/internal/domain"
	"github.com/hagile/comporta/internal/store"
)

type seedPartner struct {
	name   string
	apiKey string
}

type seedRoute struct {
	pathPrefix  string
	upstreamURL string
	stripPrefix bool
}

type seedPolicy struct {
	partnerIdx        int // indice em partners
	routeIdx          int // indice em routes, -1 significa politica padrao (route_id = 0)
	requestsPerMinute int
}

func main() {
	cfg := config.Load()
	if cfg.OracleDSN == "" {
		log.Fatal("ORACLE_DSN nao configurado: aponte para o Oracle do docker compose antes de rodar o seed")
	}

	db, err := store.OpenOracle(cfg.OracleDSN)
	if err != nil {
		log.Fatalf("falha ao conectar no oracle: %v", err)
	}
	defer db.Close()

	repos := store.NewOracleRepositories(db).AsRepositories()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	partners := []seedPartner{
		{name: "Aurora Fintech", apiKey: "cpk_demo_aurora_fintech_0001"},
		{name: "LogiFast Transportes", apiKey: "cpk_demo_logifast_transportes_0002"},
		{name: "Varejo Union", apiKey: "cpk_demo_varejo_union_0003"},
	}

	routes := []seedRoute{
		{pathPrefix: "/billing", upstreamURL: "http://mock-billing:9001", stripPrefix: true},
		{pathPrefix: "/inventory", upstreamURL: "http://mock-inventory:9002", stripPrefix: true},
	}

	policies := []seedPolicy{
		{partnerIdx: 0, routeIdx: 0, requestsPerMinute: 120}, // Aurora Fintech em /billing
		{partnerIdx: 0, routeIdx: -1, requestsPerMinute: 30}, // Aurora Fintech, padrao
		{partnerIdx: 1, routeIdx: 1, requestsPerMinute: 60},  // LogiFast em /inventory
		{partnerIdx: 2, routeIdx: -1, requestsPerMinute: 20}, // Varejo Union, padrao
	}

	partnerIDs := make([]int64, len(partners))
	for i, p := range partners {
		id, err := ensurePartner(ctx, repos.Partners, p)
		if err != nil {
			log.Fatalf("falha ao semear parceiro %q: %v", p.name, err)
		}
		partnerIDs[i] = id
		fmt.Printf("parceiro pronto: %s (id=%d, api_key=%s)\n", p.name, id, p.apiKey)
	}

	routeIDs := make([]int64, len(routes))
	for i, r := range routes {
		id, err := ensureRoute(ctx, repos.Routes, r)
		if err != nil {
			log.Fatalf("falha ao semear rota %q: %v", r.pathPrefix, err)
		}
		routeIDs[i] = id
		fmt.Printf("rota pronta: %s -> %s (id=%d)\n", r.pathPrefix, r.upstreamURL, id)
	}

	for _, pol := range policies {
		routeID := int64(0)
		if pol.routeIdx >= 0 {
			routeID = routeIDs[pol.routeIdx]
		}
		partnerID := partnerIDs[pol.partnerIdx]
		if err := ensurePolicy(ctx, repos.Policies, partnerID, routeID, pol.requestsPerMinute); err != nil {
			log.Fatalf("falha ao semear politica para parceiro id=%d: %v", partnerID, err)
		}
		fmt.Printf("politica pronta: parceiro id=%d, rota id=%d, %d req/min\n", partnerID, routeID, pol.requestsPerMinute)
	}

	fmt.Println("seed concluido")
}

func ensurePartner(ctx context.Context, repo store.PartnerRepository, p seedPartner) (int64, error) {
	if existing, err := repo.GetByAPIKey(ctx, p.apiKey); err == nil {
		return existing.ID, nil
	}
	partner := domain.Partner{Name: p.name, APIKey: p.apiKey, Active: true}
	if err := repo.Create(ctx, &partner); err != nil {
		return 0, err
	}
	return partner.ID, nil
}

func ensureRoute(ctx context.Context, repo store.RouteRepository, r seedRoute) (int64, error) {
	existing, _, err := repo.List(ctx, 1, 100)
	if err == nil {
		for _, rt := range existing {
			if rt.PathPrefix == r.pathPrefix {
				return rt.ID, nil
			}
		}
	}
	route := domain.Route{
		PathPrefix:  r.pathPrefix,
		UpstreamURL: r.upstreamURL,
		StripPrefix: r.stripPrefix,
		Active:      true,
	}
	if err := repo.Create(ctx, &route); err != nil {
		return 0, err
	}
	return route.ID, nil
}

func ensurePolicy(ctx context.Context, repo store.PolicyRepository, partnerID, routeID int64, rpm int) error {
	if existing, err := repo.FindEffective(ctx, partnerID, routeID); err == nil && existing.RouteID == routeID {
		return nil
	}
	policy := domain.RateLimitPolicy{PartnerID: partnerID, RouteID: routeID, RequestsPerMinute: rpm}
	return repo.Create(ctx, &policy)
}
