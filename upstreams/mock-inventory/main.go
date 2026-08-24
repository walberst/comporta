// Servico de exemplo que simula um sistema legado de controle de estoque.
// Assim como o mock-billing, existe so para dar ao gateway um upstream real
// para rotear no docker compose up.
package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

type stockItem struct {
	SKU      string `json:"sku"`
	Product  string `json:"product"`
	Quantity int    `json:"quantity"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9002"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/stock", listStock)

	log.Printf("mock-inventory ouvindo na porta %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func listStock(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
	if rand.Intn(25) == 0 {
		http.Error(w, "falha simulada no sistema legado de estoque", http.StatusInternalServerError)
		return
	}
	items := []stockItem{
		{SKU: "SKU-001", Product: "Caixa de transporte termica", Quantity: 320},
		{SKU: "SKU-002", Product: "Etiqueta de rastreio", Quantity: 15000},
		{SKU: "SKU-003", Product: "Palete plastico reforcado", Quantity: 48},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(items)
}
