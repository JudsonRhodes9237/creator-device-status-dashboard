package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/infrai-examples/creator-device-dashboard/internal/dashboard"
	"github.com/infrai-examples/creator-device-dashboard/internal/infrai"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := infrai.New(key)
	stream := dashboard.NewStream(client, envOr("INFRAI_CHANNEL", "creator-device-status"))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /device-updates", func(w http.ResponseWriter, r *http.Request) {
		var update dashboard.DeviceUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		result, err := stream.Send(r.Context(), update)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, result)
	})
	mux.HandleFunc("GET /dashboard/metrics", func(w http.ResponseWriter, r *http.Request) {
		name, agg := r.URL.Query().Get("name"), r.URL.Query().Get("agg")
		if name == "" || agg == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and agg are required"})
			return
		}
		data, err := client.QueryMetrics(r.Context(), name, agg)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	addr := envOr("ADDR", ":8080")
	log.Printf("device dashboard bridge listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func writeServiceError(w http.ResponseWriter, err error) {
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Message, "code": apiErr.Code})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream request failed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
