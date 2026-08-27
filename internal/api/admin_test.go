package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/walberst/comporta/internal/api"
	"github.com/walberst/comporta/internal/domain"
	"github.com/walberst/comporta/internal/store"
)

func newAdminHandler() (*api.AdminHandler, store.Repositories) {
	repos := store.NewMemoryRepositories().AsRepositories()
	return &api.AdminHandler{Repos: repos, Log: zap.NewNop()}, repos
}

func doJSON(t *testing.T, handler http.HandlerFunc, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestAdminHandler_CriaEListaParceiros(t *testing.T) {
	h, _ := newAdminHandler()

	rec := doJSON(t, h.CreatePartner, http.MethodPost, "/admin/partners", map[string]string{"name": "Aurora Fintech"})
	require.Equal(t, http.StatusCreated, rec.Code)

	var created struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		APIKey string `json:"api_key"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotZero(t, created.ID)
	require.NotEmpty(t, created.APIKey)

	rec = doJSON(t, h.ListPartners, http.MethodGet, "/admin/partners", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var listed struct {
		Data  []domain.Partner `json:"data"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listed))
	require.Equal(t, 1, listed.Total)
}

func TestAdminHandler_CriarParceiroSemNomeFalha(t *testing.T) {
	h, _ := newAdminHandler()
	rec := doJSON(t, h.CreatePartner, http.MethodPost, "/admin/partners", map[string]string{"name": ""})
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminHandler_CriaRotaComValidacao(t *testing.T) {
	h, _ := newAdminHandler()

	rec := doJSON(t, h.CreateRoute, http.MethodPost, "/admin/routes", map[string]interface{}{
		"path_prefix":  "billing",
		"upstream_url": "http://mock-billing:9001",
	})
	require.Equal(t, http.StatusBadRequest, rec.Code, "path_prefix precisa comecar com /")

	rec = doJSON(t, h.CreateRoute, http.MethodPost, "/admin/routes", map[string]interface{}{
		"path_prefix":  "/billing",
		"upstream_url": "nao-e-uma-url",
	})
	require.Equal(t, http.StatusBadRequest, rec.Code, "upstream_url precisa ser uma url valida")

	rec = doJSON(t, h.CreateRoute, http.MethodPost, "/admin/routes", map[string]interface{}{
		"path_prefix":  "/billing",
		"upstream_url": "http://mock-billing:9001",
		"strip_prefix": true,
	})
	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestAdminHandler_CriaPoliticaValidandoReferencias(t *testing.T) {
	h, repos := newAdminHandler()

	rec := doJSON(t, h.CreatePolicy, http.MethodPost, "/admin/policies", map[string]interface{}{
		"partner_id":          999,
		"requests_per_minute": 60,
	})
	require.Equal(t, http.StatusBadRequest, rec.Code, "partner_id inexistente deveria falhar")

	partner := domain.Partner{Name: "Aurora Fintech", APIKey: "cpk_x"}
	require.NoError(t, repos.Partners.Create(context.Background(), &partner))

	rec = doJSON(t, h.CreatePolicy, http.MethodPost, "/admin/policies", map[string]interface{}{
		"partner_id":          partner.ID,
		"requests_per_minute": 0,
	})
	require.Equal(t, http.StatusBadRequest, rec.Code, "requests_per_minute precisa ser positivo")

	rec = doJSON(t, h.CreatePolicy, http.MethodPost, "/admin/policies", map[string]interface{}{
		"partner_id":          partner.ID,
		"requests_per_minute": 60,
	})
	require.Equal(t, http.StatusCreated, rec.Code)
}

func TestAdminHandler_DeletarParceiroInexistenteRetorna404(t *testing.T) {
	h, _ := newAdminHandler()
	req := httptest.NewRequest(http.MethodDelete, "/admin/partners/999", nil)
	req.SetPathValue("id", "999")
	rec := httptest.NewRecorder()

	h.DeletePartner(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}
