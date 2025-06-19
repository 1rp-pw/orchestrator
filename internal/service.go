package internal

import (
	"context"
	"crypto/tls"
	"github.com/1rp-pw/orchestrator/internal/engine"
	"github.com/1rp-pw/orchestrator/internal/flow"
	"github.com/1rp-pw/orchestrator/internal/policy"
	"github.com/bugfixes/go-bugfixes/logs"
	"github.com/bugfixes/go-bugfixes/middleware"
	ConfigBuilder "github.com/keloran/go-config"
	"net/http"
	"strconv"
	"time"
)

type Service struct {
	Config *ConfigBuilder.Config
}

func New(cfg *ConfigBuilder.Config) *Service {
	return &Service{
		Config: cfg,
	}
}

func (s *Service) Start() error {
	errChan := make(chan error)
	go s.startHTTP(errChan)

	return <-errChan
}

func (s *Service) startHTTP(errChan chan error) {
	mux := http.NewServeMux()

	// run the structs on the engine
	mux.HandleFunc("POST /run", engine.NewSystem(s.Config).Run)
	mux.HandleFunc("POST /run/{policyId}", engine.NewSystem(s.Config).RunPolicy)

	// structs storage
	mux.HandleFunc("POST /policy", policy.NewSystem(s.Config).CreatePolicy)
	mux.HandleFunc("GET /policy/{policyId}/draft", policy.NewSystem(s.Config).CreateDraftFromVersion)
	mux.HandleFunc("PUT /policy/{policyId}", policy.NewSystem(s.Config).UpdatePolicy)
	mux.HandleFunc("DELETE /policy/{policyId}", policy.NewSystem(s.Config).DeletePolicy)
	mux.HandleFunc("GET /policy/{policyId}", policy.NewSystem(s.Config).GetPolicy)
	mux.HandleFunc("GET /policy/{policyId}/versions", policy.NewSystem(s.Config).ListPolicyVersions)
	mux.HandleFunc("GET /policy/{policyId}/{versionId}", policy.NewSystem(s.Config).GetPolicyVersion)
	mux.HandleFunc("GET /policies", policy.NewSystem(s.Config).GetAllPolicies)

	// flow system
	mux.HandleFunc("GET /flows", flow.NewSystem(s.Config).GetAllFlows)
	mux.HandleFunc("POST /flow", flow.NewSystem(s.Config).CreateFlow)
	mux.HandleFunc("GET /flow/{flowId}/versions", flow.NewSystem(s.Config).ListFlowVersions)

	mux.HandleFunc("GET /flow", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
	mux.HandleFunc("GET /flow/{flowId}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
	mux.HandleFunc("PUT /flow/{flowId}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
	mux.HandleFunc("DELETE /flow/{flowId}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
	mux.HandleFunc("POST /flow/test", flow.NewSystem(s.Config).TestFlow)
	mux.HandleFunc("POST /flow/{flowId}", flow.NewSystem(s.Config).RunFlow)

	mw := middleware.NewMiddleware(context.Background())
	mw.AddMiddleware(middleware.SetupLogger(middleware.Error).Logger)
	mw.AddMiddleware(middleware.RequestID)
	mw.AddMiddleware(middleware.Recoverer)
	mw.AddMiddleware(mw.CORS)
	mw.AddMiddleware(middleware.LowerCaseHeaders)
	mw.AddAllowedMethods(http.MethodGet, http.MethodPost, http.MethodOptions, http.MethodDelete, http.MethodPut)

	port := s.Config.Local.HTTPPort
	if s.Config.ProjectProperties["railway_port"].(string) != "" && s.Config.ProjectProperties["on_railway"].(bool) {
		i, err := strconv.Atoi(s.Config.ProjectProperties["railway_port"].(string))
		if err != nil {
			errChan <- logs.Error("failed to parse port %v", err)
		}
		port = i
	}

	logs.Logf("Starting server on port %d", port)
	server := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		TLSNextProto:      make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
	errChan <- server.ListenAndServe()
}
