package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

func apiListRuns(model *Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 20
		if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 100 {
			limit = n
		}
		runs, err := model.RecentRuns(limit)
		if err != nil {
			http.Error(w, "error loading runs", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(runs)
	}
}

func apiGetRun(model *Model, outputDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid run ID", http.StatusBadRequest)
			return
		}
		state, err := buildRunState(model, outputDir, id)
		if err != nil {
			http.Error(w, "run not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state)
	}
}

func apiListDeployments(model *Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deployments, err := model.CurrentDeployments()
		if err != nil {
			http.Error(w, "error loading deployments", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deployments)
	}
}
