package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/walberst/comporta/internal/domain"
	"github.com/walberst/comporta/internal/store"
)

func TestMemoryRepositories_PaginacaoDeParceiros(t *testing.T) {
	repos := store.NewMemoryRepositories().AsRepositories()
	ctx := context.Background()

	for i := 0; i < 25; i++ {
		p := domain.Partner{Name: "parceiro", APIKey: "key-" + string(rune('a'+i))}
		require.NoError(t, repos.Partners.Create(ctx, &p))
	}

	page1, total, err := repos.Partners.List(ctx, 1, 10)
	require.NoError(t, err)
	require.Equal(t, 25, total)
	require.Len(t, page1, 10)

	page3, _, err := repos.Partners.List(ctx, 3, 10)
	require.NoError(t, err)
	require.Len(t, page3, 5, "ultima pagina deveria trazer o resto")
}

func TestMemoryRepositories_PolicyFindEffective(t *testing.T) {
	repos := store.NewMemoryRepositories().AsRepositories()
	ctx := context.Background()

	partner := domain.Partner{Name: "Aurora", APIKey: "k1"}
	require.NoError(t, repos.Partners.Create(ctx, &partner))
	route := domain.Route{PathPrefix: "/billing", UpstreamURL: "http://x"}
	require.NoError(t, repos.Routes.Create(ctx, &route))

	// Sem nenhuma politica cadastrada, deve falhar.
	_, err := repos.Policies.FindEffective(ctx, partner.ID, route.ID)
	require.ErrorIs(t, err, store.ErrNotFound)

	defaultPolicy := domain.RateLimitPolicy{PartnerID: partner.ID, RouteID: 0, RequestsPerMinute: 10}
	require.NoError(t, repos.Policies.Create(ctx, &defaultPolicy))

	found, err := repos.Policies.FindEffective(ctx, partner.ID, route.ID)
	require.NoError(t, err)
	require.Equal(t, 10, found.RequestsPerMinute, "deveria cair na politica padrao do parceiro")

	specificPolicy := domain.RateLimitPolicy{PartnerID: partner.ID, RouteID: route.ID, RequestsPerMinute: 99}
	require.NoError(t, repos.Policies.Create(ctx, &specificPolicy))

	found, err = repos.Policies.FindEffective(ctx, partner.ID, route.ID)
	require.NoError(t, err)
	require.Equal(t, 99, found.RequestsPerMinute, "a politica especifica da rota deveria vencer a padrao")
}

func TestMemoryRepositories_DeleteInexistenteRetornaErro(t *testing.T) {
	repos := store.NewMemoryRepositories().AsRepositories()
	err := repos.Partners.Delete(context.Background(), 999)
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemoryRepositories_TopConsumers(t *testing.T) {
	repos := store.NewMemoryRepositories().AsRepositories()
	ctx := context.Background()

	partner := domain.Partner{Name: "Aurora", APIKey: "k1"}
	require.NoError(t, repos.Partners.Create(ctx, &partner))
	route := domain.Route{PathPrefix: "/billing", UpstreamURL: "http://x"}
	require.NoError(t, repos.Routes.Create(ctx, &route))

	for i := 0; i < 3; i++ {
		require.NoError(t, repos.Audit.Insert(ctx, &domain.AuditLog{
			PartnerID: partner.ID, RouteID: route.ID, Method: "GET", Path: "/billing", StatusCode: 200,
		}))
	}

	top, err := repos.Audit.TopConsumers(ctx, time.Now().Add(-time.Hour), 5)
	require.NoError(t, err)
	require.Len(t, top, 1)
	require.Equal(t, int64(3), top[0].Requests)
	require.Equal(t, "Aurora", top[0].PartnerName)
}
