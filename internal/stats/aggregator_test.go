package stats_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hagile/comporta/internal/stats"
)

func TestAggregator_CalculaTaxaDeErroERanking(t *testing.T) {
	agg := stats.NewAggregator()

	agg.Record(1, "Aurora Fintech", 10, "/billing", 200)
	agg.Record(1, "Aurora Fintech", 10, "/billing", 200)
	agg.Record(1, "Aurora Fintech", 10, "/billing", 500)
	agg.Record(2, "LogiFast", 11, "/inventory", 429)

	snap := agg.Snapshot(10)

	require.InDelta(t, 0.5, snap.ErrorRate, 0.001, "2 de 4 requisicoes com status >= 400")
	require.Len(t, snap.TopConsumers, 2)
	require.Equal(t, int64(1), snap.TopConsumers[0].PartnerID)
	require.Equal(t, int64(3), snap.TopConsumers[0].Requests)
}

func TestAggregator_SnapshotReiniciaJanela(t *testing.T) {
	agg := stats.NewAggregator()
	agg.Record(1, "Aurora Fintech", 10, "/billing", 200)

	first := agg.Snapshot(10)
	require.Len(t, first.TopConsumers, 1)

	second := agg.Snapshot(10)
	require.Empty(t, second.TopConsumers, "a segunda leitura nao deveria repetir contagem da primeira janela")
	require.Equal(t, 0.0, second.ErrorRate)
}

func TestAggregator_RespeitaLimiteDoRanking(t *testing.T) {
	agg := stats.NewAggregator()
	for i := int64(1); i <= 5; i++ {
		agg.Record(i, "parceiro", i, "/rota", 200)
	}

	snap := agg.Snapshot(3)
	require.Len(t, snap.TopConsumers, 3)
}
