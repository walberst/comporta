// Package logging configura o logger estruturado (Zap) usado em todo o
// gateway. Logs estruturados em JSON facilitam agregacao em qualquer stack
// de observabilidade (Loki, ELK, CloudWatch) sem parser customizado.
package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New cria um logger Zap. Em "debug" usa o encoder de console (mais legivel
// durante desenvolvimento local); em qualquer outro nivel usa JSON, formato
// esperado pelos coletores de log em producao.
func New(level string) (*zap.Logger, error) {
	var cfg zap.Config
	if level == "debug" {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}

	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		lvl = zapcore.InfoLevel
	}
	cfg.Level = zap.NewAtomicLevelAt(lvl)

	return cfg.Build()
}
