package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hagile/comporta/internal/ratelimit"
)

// newTestLimiter sobe um Redis em memoria (miniredis) para o teste de
// integracao do rate limiter nao depender de um Redis real rodando na
// maquina que executa o CI.
func newTestLimiter(t *testing.T) (*ratelimit.Limiter, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	return ratelimit.NewLimiter(rdb), mr
}

func TestAllow_PermiteDentroDaCota(t *testing.T) {
	limiter, _ := newTestLimiter(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		res, err := limiter.Allow(ctx, "partner:1:route:1", 5, time.Minute)
		require.NoError(t, err)
		require.True(t, res.Allowed, "requisicao %d deveria ser permitida", i+1)
	}
}

func TestAllow_BloqueiaAposEstourarCota(t *testing.T) {
	limiter, _ := newTestLimiter(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		res, err := limiter.Allow(ctx, "partner:2:route:1", 3, time.Minute)
		require.NoError(t, err)
		require.True(t, res.Allowed)
	}

	res, err := limiter.Allow(ctx, "partner:2:route:1", 3, time.Minute)
	require.NoError(t, err)
	require.False(t, res.Allowed, "a quarta requisicao deveria estourar a cota de 3 por minuto")
	require.Greater(t, res.RetryAfter, time.Duration(0))
}

func TestAllow_JanelaDeslizanteLiberaAposExpirar(t *testing.T) {
	limiter, _ := newTestLimiter(t)
	ctx := context.Background()

	res, err := limiter.Allow(ctx, "partner:3:route:1", 1, 300*time.Millisecond)
	require.NoError(t, err)
	require.True(t, res.Allowed)

	res, err = limiter.Allow(ctx, "partner:3:route:1", 1, 300*time.Millisecond)
	require.NoError(t, err)
	require.False(t, res.Allowed)

	// A janela usa o relogio real (time.Now no lado do Go), entao para
	// verificar a liberacao apos expirar precisamos esperar de verdade; a
	// janela curta de 300ms mantem o teste rapido.
	time.Sleep(350 * time.Millisecond)

	res, err = limiter.Allow(ctx, "partner:3:route:1", 1, 300*time.Millisecond)
	require.NoError(t, err)
	require.True(t, res.Allowed, "apos a janela expirar a cota deveria estar disponivel de novo")
}

func TestAllow_ChavesDiferentesNaoInterferemEntreSi(t *testing.T) {
	limiter, _ := newTestLimiter(t)
	ctx := context.Background()

	res, err := limiter.Allow(ctx, "partner:4:route:1", 1, time.Minute)
	require.NoError(t, err)
	require.True(t, res.Allowed)

	res, err = limiter.Allow(ctx, "partner:4:route:1", 1, time.Minute)
	require.NoError(t, err)
	require.False(t, res.Allowed)

	res, err = limiter.Allow(ctx, "partner:4:route:2", 1, time.Minute)
	require.NoError(t, err)
	require.True(t, res.Allowed, "uma rota diferente do mesmo parceiro tem cota propria")
}
