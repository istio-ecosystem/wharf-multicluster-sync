package agent

import (
	"encoding/json"
	"fmt"
	"net/http"

	istiomodel "istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/log"
)

// RenderJSON outputs the given data as JSON
func RenderJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if err := json.NewEncoder(w).Encode(data); err != nil {
		RenderError(w, http.StatusInternalServerError, err)
	}
}

// RenderError outputs an error message
func RenderError(w http.ResponseWriter, statusCode int, err error) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "%v", err)
}

// PrintReconcileAddResults print to logger the results of calling the AddMulticlusterConfig
func PrintReconcileAddResults(added []istiomodel.Config, modified []istiomodel.Config, err error) {
	if err != nil {
		log.Errora(err)
		return
	}
	log.Debugf("Istio configs added: %d", len(added))
	for _, cfg := range added {
		log.Debugf("\tType:%s\tName: %s", cfg.Type, cfg.Name)
	}
	log.Debugf("Istio configs modified: %d", len(modified))
	for _, cfg := range modified {
		log.Debugf("\tType:%s\tName: %s", cfg.Type, cfg.Name)
	}
}

// PrintReconcileDeleteResults print to logger the results of calling the DeleteMulticlusterConfig
func PrintReconcileDeleteResults(deleted []istiomodel.Config, err error) {
	if err != nil {
		log.Errora(err)
		return
	}
	log.Debugf("Istio configs deleted: %d", len(deleted))
	for _, cfg := range deleted {
		log.Debugf("\tType:%s\tName: %s", cfg.Type, cfg.Name)
	}
}
