package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// validationError e usado para diferenciar erro de entrada invalida (400) de
// erro interno (500) sem a camada de handler precisar conhecer regra de
// negocio especifica de cada entidade.
type validationError struct{ msg string }

func (e validationError) Error() string { return e.msg }

func requireNonEmpty(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return validationError{fmt.Sprintf("%s e obrigatorio", field)}
	}
	return nil
}

func requirePathPrefix(value string) error {
	if !strings.HasPrefix(value, "/") {
		return validationError{"path_prefix precisa comecar com /"}
	}
	return nil
}

func requireValidURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return validationError{"upstream_url precisa ser uma url absoluta valida (http ou https)"}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return validationError{"upstream_url precisa usar http ou https"}
	}
	return nil
}

func requirePositive(field string, value int) error {
	if value <= 0 {
		return validationError{fmt.Sprintf("%s precisa ser maior que zero", field)}
	}
	return nil
}

// generateAPIKey cria uma chave aleatoria para um novo parceiro. O prefixo
// facilita reconhecer visualmente uma chave do Comporta em logs e paineis de
// terceiros sem precisar decodificar nada.
func generateAPIKey() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "cpk_" + hex.EncodeToString(buf), nil
}
