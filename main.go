package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"
)

type announcementHandler struct {
	publisher Publisher
}

func (h announcementHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var input Announcement
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid announcement"})
		return
	}
	if _, err := input.event(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		sum := sha256.Sum256([]byte(input.Kind + "\x00" + input.CourseID + "\x00" + input.Title + "\x00" + input.DueAt.UTC().Format(time.RFC3339Nano)))
		key = hex.EncodeToString(sum[:])
	}
	delivery, err := h.publisher.Publish(r.Context(), input, key)
	if err != nil {
		status := http.StatusBadGateway
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
			status = apiErr.StatusCode
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, delivery)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	channel := os.Getenv("INFRAI_EDTECH_CHANNEL")
	if apiKey == "" || channel == "" {
		log.Fatal("INFRAI_API_KEY and INFRAI_EDTECH_CHANNEL are required")
	}

	announcements := announcementHandler{publisher: Publisher{
		APIKey: apiKey, Channel: channel, MaxRetries: 3,
	}}
	mux := http.NewServeMux()
	mux.Handle("/announcements", announcements)
	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("announcement relay listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}
