// Package wshub implementa o hub de websocket que empurra as metricas ao
// vivo (requisicoes por segundo, taxa de erro, ranking de consumidores) para
// o painel Nuxt. Usa gorilla/websocket diretamente, sem framework de
// realtime por cima, porque o volume de mensagens e baixo (um snapshot por
// segundo) e nao justifica a complexidade extra.
package wshub

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/walberst/comporta/internal/metrics"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// O painel roda em outra origem durante desenvolvimento (Nuxt dev server
	// em porta diferente do gateway), entao liberamos qualquer origem aqui.
	// Em producao o admin token e a api key ja protegem os dados sensiveis;
	// o canal de metricas agregadas nao expoe segredos.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Hub mantem a lista de clientes conectados e distribui cada snapshot
// recebido para todos eles.
type Hub struct {
	log *zap.Logger

	mu      sync.Mutex
	clients map[*client]struct{}
}

type client struct {
	conn *websocket.Conn
	send chan []byte
}

func NewHub(log *zap.Logger) *Hub {
	return &Hub{log: log, clients: make(map[*client]struct{})}
}

// ServeWS faz o upgrade da conexao HTTP para websocket e mantem o cliente
// registrado ate a conexao cair.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("falha no upgrade do websocket", zap.Error(err))
		return
	}

	c := &client{conn: conn, send: make(chan []byte, 16)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	metrics.WSClientsConnected.Inc()

	go h.writePump(c)
	h.readPump(c)
}

// readPump so existe para detectar o fechamento da conexao pelo cliente
// (o protocolo e unidirecional: o gateway so envia, nunca espera comandos).
func (h *Hub) readPump(c *client) {
	defer h.disconnect(c)
	c.conn.SetReadLimit(512)
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Hub) writePump(c *client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (h *Hub) disconnect(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
	metrics.WSClientsConnected.Dec()
	_ = c.conn.Close()
}

// Broadcast serializa o payload em JSON e entrega para todos os clientes
// conectados. Clientes lentos (buffer cheio) sao desconectados em vez de
// travar o broadcast para todo mundo.
func (h *Hub) Broadcast(payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		h.log.Error("falha ao serializar snapshot para broadcast", zap.Error(err))
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			delete(h.clients, c)
			close(c.send)
			go func(conn *websocket.Conn) { _ = conn.Close() }(c.conn)
		}
	}
}
