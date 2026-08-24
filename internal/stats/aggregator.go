// Package stats mantem um resumo em memoria do trafego recente do gateway
// (requisicoes por segundo, taxa de erro e ranking de consumidores). Isso
// alimenta o websocket de metricas ao vivo sem precisar consultar o Oracle
// a cada tick, o que seria caro e desnecessario para um numero que so
// precisa ser "aproximadamente agora".
package stats

import (
	"sort"
	"strconv"
	"sync"
	"time"
)

// ConsumerCount e o consumo de um parceiro numa rota dentro da janela atual.
type ConsumerCount struct {
	PartnerID   int64  `json:"partner_id"`
	PartnerName string `json:"partner_name"`
	RouteID     int64  `json:"route_id"`
	RoutePath   string `json:"route_path"`
	Requests    int64  `json:"requests"`
}

// Snapshot e o pacote de metricas enviado para o painel a cada intervalo.
type Snapshot struct {
	Timestamp         time.Time       `json:"timestamp"`
	RequestsPerSecond float64         `json:"requests_per_second"`
	ErrorRate         float64         `json:"error_rate"`
	TopConsumers      []ConsumerCount `json:"top_consumers"`
}

// Aggregator acumula contadores desde o ultimo snapshot e os zera a cada
// chamada de Snapshot, formando janelas consecutivas (nao deslizantes: o
// custo de uma janela deslizante aqui nao se paga, ja que o painel so
// precisa de uma leitura por segundo, nao de precisao historica).
type Aggregator struct {
	mu          sync.Mutex
	windowStart time.Time
	total       int64
	errors      int64
	consumers   map[string]*ConsumerCount
}

func NewAggregator() *Aggregator {
	return &Aggregator{
		windowStart: time.Now(),
		consumers:   make(map[string]*ConsumerCount),
	}
}

// Record contabiliza uma requisicao concluida.
func (a *Aggregator) Record(partnerID int64, partnerName string, routeID int64, routePath string, statusCode int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.total++
	if statusCode >= 400 {
		a.errors++
	}

	key := consumerKey(partnerID, routeID)
	c, ok := a.consumers[key]
	if !ok {
		c = &ConsumerCount{PartnerID: partnerID, PartnerName: partnerName, RouteID: routeID, RoutePath: routePath}
		a.consumers[key] = c
	}
	c.Requests++
}

// Snapshot calcula as metricas da janela corrente e reinicia os contadores
// para a proxima janela. topN limita o tamanho do ranking de consumidores.
func (a *Aggregator) Snapshot(topN int) Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(a.windowStart).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}

	rps := float64(a.total) / elapsed
	errorRate := 0.0
	if a.total > 0 {
		errorRate = float64(a.errors) / float64(a.total)
	}

	top := make([]ConsumerCount, 0, len(a.consumers))
	for _, c := range a.consumers {
		top = append(top, *c)
	}
	sort.Slice(top, func(i, j int) bool { return top[i].Requests > top[j].Requests })
	if len(top) > topN {
		top = top[:topN]
	}

	snapshot := Snapshot{
		Timestamp:         now,
		RequestsPerSecond: rps,
		ErrorRate:         errorRate,
		TopConsumers:      top,
	}

	a.windowStart = now
	a.total = 0
	a.errors = 0
	a.consumers = make(map[string]*ConsumerCount)

	return snapshot
}

func consumerKey(partnerID, routeID int64) string {
	// Concatenacao simples e suficiente: os dois IDs numericos nunca colidem
	// em formato texto porque o separador nao aparece nos numeros.
	return strconv.FormatInt(partnerID, 10) + ":" + strconv.FormatInt(routeID, 10)
}
