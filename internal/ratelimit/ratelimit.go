// Package ratelimit implementa limitacao de taxa por janela deslizante
// usando Redis. A janela deslizante foi escolhida em vez de um contador fixo
// por minuto porque o contador fixo permite um parceiro dobrar sua cota real
// disparando rajadas nos dois lados da virada do minuto (ex: 60 requisicoes
// no segundo 59 e mais 60 no segundo 1 do minuto seguinte).
package ratelimit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// script mantem a leitura, a limpeza da janela e o registro da nova
// requisicao atomicos dentro do Redis, evitando condicao de corrida entre
// duas instancias do gateway decidindo ao mesmo tempo se ainda ha cota.
const script = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

redis.call('ZREMRANGEBYSCORE', key, 0, now - window_ms)
local count = redis.call('ZCARD', key)

if count < limit then
    redis.call('ZADD', key, now, member)
    redis.call('PEXPIRE', key, window_ms)
    return {1, 0}
end

local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
local retry_after_ms = window_ms
if oldest[2] ~= nil then
    retry_after_ms = window_ms - (now - tonumber(oldest[2]))
end
if retry_after_ms < 0 then
    retry_after_ms = 0
end
return {0, retry_after_ms}
`

// Result e a resposta de uma verificacao de cota.
type Result struct {
	Allowed    bool
	RetryAfter time.Duration
}

// Limiter aplica a janela deslizante contra um cliente Redis compartilhado
// entre todas as instancias do gateway.
type Limiter struct {
	rdb    *redis.Client
	script *redis.Script
}

func NewLimiter(rdb *redis.Client) *Limiter {
	return &Limiter{rdb: rdb, script: redis.NewScript(script)}
}

// Allow verifica se a chave (tipicamente parceiro+rota) ainda tem cota
// dentro da janela informada. limit e o numero maximo de requisicoes
// permitidas dentro de window.
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (Result, error) {
	if limit <= 0 {
		return Result{Allowed: false, RetryAfter: window}, nil
	}
	member, err := randomMember()
	if err != nil {
		return Result{}, fmt.Errorf("gerando membro unico para rate limit: %w", err)
	}
	now := time.Now().UnixMilli()
	windowMs := window.Milliseconds()

	res, err := l.script.Run(ctx, l.rdb, []string{key}, now, windowMs, limit, member).Result()
	if err != nil {
		return Result{}, fmt.Errorf("executando script de rate limit: %w", err)
	}

	values, ok := res.([]interface{})
	if !ok || len(values) != 2 {
		return Result{}, fmt.Errorf("resposta inesperada do script de rate limit: %v", res)
	}
	allowed, _ := values[0].(int64)
	retryMs, _ := values[1].(int64)

	return Result{
		Allowed:    allowed == 1,
		RetryAfter: time.Duration(retryMs) * time.Millisecond,
	}, nil
}

func randomMember() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
