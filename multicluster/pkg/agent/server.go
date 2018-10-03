package agent

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/istio-ecosystem/wharf-multicluster-sync/api/multicluster/v1alpha1"
	mcmodel "github.com/istio-ecosystem/wharf-multicluster-sync/multicluster/pkg/model"

	"istio.io/istio/pkg/log"

	"github.com/gorilla/mux"
)

// Server is an agent server meant to listen on a specific port and serve
// requests coming from client agents on remote clusters.
type Server struct {
	httpServer http.Server
	store      mcmodel.MCConfigStore
	config     *ClusterConfig
}

// NewServer will create a new agent server to serve peer request on the
// specific address:port with information from the provided config store. The
// server will start listening only when the Run() function is called.
func NewServer(config *ClusterConfig, store mcmodel.MCConfigStore) (*Server, error) {
	router := mux.NewRouter()
	s := &Server{
		httpServer: http.Server{
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			Addr:         fmt.Sprintf(":%d", config.AgentPort),
			Handler:      router,
		},
		store:  store,
		config: config,
	}
	_ = router.NewRoute().PathPrefix("/exposed/{clusterID}").Methods("GET").HandlerFunc(s.handlePoliciesReq)

	return s, nil
}

// Run will start listening and serving requests in a go routine
func (s *Server) Run() {
	go func() {
		// start serving
		if err := s.httpServer.ListenAndServe(); err != nil {
			log.Errora(err)
		}
	}()
}

// Close cleans up resources used by the server.
func (s *Server) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Errora("Failed to shutdown the HTTP server", err)
	}
	cancel()
	log.Debug("Agent server closed")
}

// Handler function to handle HTTP requests for cluster policies. The function
// will find the relevant information for the caller cluster and write the
// HTTP response as a JSON object. If cluster is not identified or any other
// error occurrs, an error JSON will be written back.
func (s *Server) handlePoliciesReq(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	clusterID := vars["clusterID"]

	if !s.isTrustedCluster(clusterID) {
		err := fmt.Errorf("operation can not be completed. Cluster not identified")
		RenderError(w, http.StatusForbidden, err)
		return
	}

	services := s.exposedServices(clusterID)
	result := &ExposedServices{
		Services: services,
	}
	RenderJSON(w, http.StatusOK, result)
}

// Function checkes whether the provided cluster identity is trusted or not.
func (s *Server) isTrustedCluster(clusterID string) bool {
	for _, trusted := range s.config.TrustedPeers {
		if trusted == clusterID || trusted == "*" {
			return true
		}
	}
	return false
}

// Search the config store for relevant services that are exposed to the
// specified cluster ID and return those.
func (s *Server) exposedServices(clusterID string) []*ExposedService {
	var results []*ExposedService
	for _, policy := range s.store.ServiceExpositionPolicies() {
		value, _ := policy.Spec.(*v1alpha1.ServiceExpositionPolicy)
		for _, exposed := range value.Exposed {
			if isRelevantExposedService(exposed, clusterID) {
				exposedName := exposed.Alias
				if exposedName == "" {
					exposedName = exposed.Name
				}
				results = append(results, &ExposedService{
					Name:      exposedName,
					Namespace: policy.Namespace,
					Port:      exposed.Port,
				})
			}
		}
	}
	return results
}

// Checks whether the cluster ID is listed in the list of clusters that the
// service is exposed to.
func isRelevantExposedService(service *v1alpha1.ServiceExpositionPolicy_ExposedService, toClusterID string) bool {
	// If there is no clusters list, we treat this policy as exposed to all trusted clusters
	if len(service.Clusters) == 0 {
		return true
	}

	// Go through the list of allowed clusters and see if it is listed
	for _, cluster := range service.Clusters {
		if cluster == toClusterID {
			return true
		}
	}

	// Service is not exposed to the specified cluster
	return false
}
