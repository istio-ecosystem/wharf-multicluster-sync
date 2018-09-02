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
func StoreIstioConfigs(store istiomodel.ConfigStore, create []istiomodel.Config, update []istiomodel.Config, delete []istiomodel.Config) {
	if len(update) > 0 {
		log.Debugf("Istio configs updated: %d", len(update))
		for _, cfg := range update {
			_, err := store.Update(cfg)
			if err != nil {
				log.Warnf("\tType:%s\tName: %s.%s [Error]", cfg.Type, cfg.Name, cfg.Namespace)
				continue
			}
			log.Debugf("\tType:%s\tName: %s.%s [Success]", cfg.Type, cfg.Name, cfg.Namespace)
		}
	}
	if len(create) > 0 {
		log.Debugf("Istio configs created: %d", len(create))
		for _, cfg := range create {
			_, err := store.Create(cfg)
			if err != nil {
				log.Warnf("\tType:%s\tName: %s.%s [Error]", cfg.Type, cfg.Name, cfg.Namespace)
				continue
			}
			log.Debugf("\tType:%s\tName: %s.%s [Success]", cfg.Type, cfg.Name, cfg.Namespace)
		}
	}
	if len(delete) > 0 {
		log.Debugf("Istio configs deleted: %d", len(delete))
		for _, cfg := range delete {
			err := store.Delete(cfg.Type, cfg.Name, cfg.Namespace)
			if err != nil {
				log.Warnf("\tType:%s\tName: %s.%s [Error]", cfg.Type, cfg.Name, cfg.Namespace)
				continue
			}
			log.Debugf("\tType:%s\tName: %s.%s [Success]", cfg.Type, cfg.Name, cfg.Namespace)
		}
	}
}
