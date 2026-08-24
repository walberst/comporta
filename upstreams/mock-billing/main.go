// Servico de exemplo que simula um sistema legado de faturamento. Existe
// apenas para o gateway ter um upstream real para rotear durante o
// docker compose up; nao faz parte do produto em si.
package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

type invoice struct {
	ID       string  `json:"id"`
	Customer string  `json:"customer"`
	Amount   float64 `json:"amount"`
	Status   string  `json:"status"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "9001"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/invoices", listInvoices)
	mux.HandleFunc("/invoices/", getInvoice)

	log.Printf("mock-billing ouvindo na porta %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func listInvoices(w http.ResponseWriter, r *http.Request) {
	// Latencia e falha simuladas de proposito, para o painel de status ter
	// algo real para mostrar em requisicoes por segundo e taxa de erro.
	time.Sleep(time.Duration(rand.Intn(80)) * time.Millisecond)
	if rand.Intn(20) == 0 {
		http.Error(w, "falha simulada no sistema legado de faturamento", http.StatusInternalServerError)
		return
	}
	invoices := []invoice{
		{ID: "INV-1001", Customer: "Aurora Fintech", Amount: 1520.40, Status: "pago"},
		{ID: "INV-1002", Customer: "LogiFast Transportes", Amount: 890.00, Status: "pendente"},
		{ID: "INV-1003", Customer: "Varejo Union", Amount: 4310.75, Status: "atrasado"},
	}
	writeJSON(w, invoices)
}

func getInvoice(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Duration(rand.Intn(80)) * time.Millisecond)
	writeJSON(w, invoice{ID: "INV-1001", Customer: "Aurora Fintech", Amount: 1520.40, Status: "pago"})
}

func writeJSON(w http.ResponseWriter, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}
