package inventory

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// Handler exposes the store over a small REST API.
//
//	GET    /health
//	GET    /devices
//	POST   /devices
//	GET    /devices/{id}
//	DELETE /devices/{id}
//	PUT    /devices/{id}/status
func Handler(s *Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /devices", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.List())
	})
	mux.HandleFunc("POST /devices", func(w http.ResponseWriter, r *http.Request) {
		var d Device
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
			writeErr(w, http.StatusBadRequest, "bad json")
			return
		}
		out, err := s.Create(d)
		if err != nil {
			writeErr(w, status(err), err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, out)
	})
	mux.HandleFunc("GET /devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		d, err := s.Get(r.PathValue("id"))
		if err != nil {
			writeErr(w, status(err), err.Error())
			return
		}
		writeJSON(w, http.StatusOK, d)
	})
	mux.HandleFunc("DELETE /devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Delete(r.PathValue("id")); err != nil {
			writeErr(w, status(err), err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /devices/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Status string `json:"status"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil || dec.Decode(&struct{}{}) != io.EOF {
			writeErr(w, http.StatusBadRequest, "bad json")
			return
		}
		out, err := s.SetStatus(r.PathValue("id"), body.Status)
		if err != nil {
			writeErr(w, status(err), err.Error())
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
	return mux
}

func status(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrExists):
		return http.StatusConflict
	case errors.Is(err, ErrInvalid):
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
