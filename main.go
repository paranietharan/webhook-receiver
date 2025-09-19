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

type StoredWebhook struct {
	ID       int         `json:"id"`
	Payload  interface{} `json:"payload"`
	Received time.Time   `json:"received"`
}

type WebhookStore struct {
	mu       sync.RWMutex
	webhooks []StoredWebhook
	nextID   int
	maxSize  int
}

var store = &WebhookStore{
	webhooks: make([]StoredWebhook, 0),
	nextID:   1,
	maxSize:  5,
}

func main() {
	http.HandleFunc("/webhook", webhookHandler)
	http.HandleFunc("/webhooks", getWebhooksHandler)
	http.HandleFunc("/webhooks/", getWebhookByIDHandler)
	http.HandleFunc("/webhooks/clear", clearWebhooksHandler)

	fmt.Println("Webhook server listening on :8080...")
	fmt.Println("Stack-based storage: Maximum 5 webhooks (LIFO)")
	fmt.Println("Endpoints:")
	fmt.Println("  POST /webhook - Receive webhooks")
	fmt.Println("  GET /webhooks - Get all webhooks (most recent first)")
	fmt.Println("  GET /webhooks/{id} - Get webhook by ID")

	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Store incoming webhooks (stack behavior - LIFO with max size)
func (ws *WebhookStore) Add(payload interface{}) int {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	storedWebhook := StoredWebhook{
		ID:       ws.nextID,
		Payload:  payload,
		Received: time.Now(),
	}

	ws.webhooks = append(ws.webhooks, storedWebhook)
	currentID := ws.nextID
	ws.nextID++

	if len(ws.webhooks) > ws.maxSize {
		ws.webhooks = ws.webhooks[1:]
	}

	return currentID
}

func (ws *WebhookStore) GetAll() []StoredWebhook {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	result := make([]StoredWebhook, len(ws.webhooks))
	for i, j := 0, len(ws.webhooks)-1; i < len(ws.webhooks); i, j = i+1, j-1 {
		result[i] = ws.webhooks[j]
	}
	return result
}

func (ws *WebhookStore) GetByID(id int) (StoredWebhook, bool) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()

	for _, webhook := range ws.webhooks {
		if webhook.ID == id {
			return webhook, true
		}
	}
	return StoredWebhook{}, false
}

func getStringFromPayload(payload interface{}, key string) string {
	if payloadMap, ok := payload.(map[string]interface{}); ok {
		if value, exists := payloadMap[key]; exists {
			if strValue, ok := value.(string); ok {
				return strValue
			}
		}
	}
	return ""
}

func getInt64FromPayload(payload interface{}, key string) int64 {
	if payloadMap, ok := payload.(map[string]interface{}); ok {
		if value, exists := payloadMap[key]; exists {
			switch v := value.(type) {
			case int64:
				return v
			case int:
				return int64(v)
			case float64:
				return int64(v)
			}
		}
	}
	return 0
}

func (ws *WebhookStore) Clear() int {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	count := len(ws.webhooks)
	ws.webhooks = make([]StoredWebhook, 0)
	ws.nextID = 1

	return count
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload interface{}
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	assignedID := store.Add(payload)

	event := getStringFromPayload(payload, "event")
	timestamp := getInt64FromPayload(payload, "timestamp")

	fmt.Printf("Stored webhook with ID: %d\n", assignedID)
	if event != "" {
		fmt.Printf("Event: %s\n", event)
	}
	if timestamp != 0 {
		fmt.Printf("Timestamp: %d\n", timestamp)
	}
	fmt.Printf("Full payload: %+v\n", payload)

	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{
		"message": "Webhook received and stored successfully",
		"id":      assignedID,
	}
	json.NewEncoder(w).Encode(response)
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

func clearWebhooksHandler(w http.ResponseWriter, r *http.Request) {
	clearedCount := store.Clear()

	fmt.Printf("Cleared all webhooks. Total cleared: %d\n", clearedCount)

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"message":       "All webhooks cleared successfully",
		"cleared_count": clearedCount,
	}
	json.NewEncoder(w).Encode(response)
}
