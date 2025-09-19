package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type WebhookPayload struct {
	ID        int                    `json:"id"`
	Event     string                 `json:"event"`
	Data      map[string]interface{} `json:"data"`
	Timestamp int64                  `json:"timestamp"`
	Received  time.Time              `json:"received"`
}

type WebhookStore struct {
	mu       sync.RWMutex
	webhooks []WebhookPayload
	nextID   int
}

var store = &WebhookStore{
	webhooks: make([]WebhookPayload, 0),
	nextID:   1,
}

func main() {
	http.HandleFunc("/webhook", webhookHandler)
	http.HandleFunc("/webhooks", getWebhooksHandler)
	http.HandleFunc("/webhooks/", getWebhookByIDHandler)

	fmt.Println("Webhook server listening on :8080...")
	fmt.Println("Endpoints:")
	fmt.Println("  POST /webhook - Receive webhooks")
	fmt.Println("  GET /webhooks - Get all webhooks")
	fmt.Println("  GET /webhooks/{id} - Get webhook by ID")

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (ws *WebhookStore) Add(payload WebhookPayload) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	payload.ID = ws.nextID
	payload.Received = time.Now()
	ws.webhooks = append(ws.webhooks, payload)
	ws.nextID++
}

func (ws *WebhookStore) GetAll() []WebhookPayload {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	result := make([]WebhookPayload, len(ws.webhooks))
	copy(result, ws.webhooks)
	return result
}

func (ws *WebhookStore) GetByID(id int) (WebhookPayload, bool) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	for _, webhook := range ws.webhooks {
		if webhook.ID == id {
			return webhook, true
		}
	}
	return WebhookPayload{}, false
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload WebhookPayload
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Store the webhook
	store.Add(payload)

	// Process the webhook
	fmt.Printf("Received webhook event: %s\n", payload.Event)
	fmt.Printf("Payload data: %+v\n", payload.Data)
	fmt.Printf("Timestamp: %d\n", payload.Timestamp)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Webhook received and stored successfully"))
}

func getWebhooksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	webhooks := store.GetAll()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":    len(webhooks),
		"webhooks": webhooks,
	})
}

func getWebhookByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	if len(path) < 10 {
		http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
		return
	}

	idStr := path[10:]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid webhook ID", http.StatusBadRequest)
		return
	}

	webhook, found := store.GetByID(id)
	if !found {
		http.Error(w, "Webhook not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(webhook)
}
