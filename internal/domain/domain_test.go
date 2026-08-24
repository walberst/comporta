package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hagile/comporta/internal/domain"
)

func TestNormalizePageParams(t *testing.T) {
	cases := []struct {
		name             string
		page, pageSize   int
		wantP, wantSize  int
	}{
		{"valores validos passam direto", 2, 10, 2, 10},
		{"pagina zero vira 1", 0, 10, 1, 10},
		{"pagina negativa vira 1", -5, 10, 1, 10},
		{"page_size zero vira padrao 20", 1, 0, 1, 20},
		{"page_size acima do teto e limitado a 100", 1, 500, 1, 100},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotP, gotSize := domain.NormalizePageParams(tc.page, tc.pageSize)
			require.Equal(t, tc.wantP, gotP)
			require.Equal(t, tc.wantSize, gotSize)
		})
	}
}
