// Package broker provides the contract broker service.
package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gabrielrauch/covenant/pkg/broker/brokerapi"
	"github.com/gabrielrauch/covenant/pkg/broker/storage"
	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/log"
)

// maxTags is the maximum number of tags allowed in a single request.
const maxTags = 50

// errInvalidStatus is returned when an invalid status value is provided.
var errInvalidStatus = errors.New("invalid status value")

// validateStatus validates and returns the status if valid.
func validateStatus(s string) (contract.Status, error) {
	status := contract.Status(s)
	switch status {
	case contract.StatusDraft, contract.StatusPublished, contract.StatusDeprecated:
		return status, nil
	default:
		return "", errInvalidStatus
	}
}

// Server is the contract broker HTTP server.
type Server struct {
	storage       storage.Backend
	contracts     *brokerapi.ContractService
	verifications *brokerapi.VerificationService
	deploy        *brokerapi.DeployService
	logger        log.Logger

	mux *http.ServeMux
}

// NewServer creates a new broker server.
func NewServer(backend storage.Backend) *Server {
	contracts := brokerapi.NewContractService(backend)
	verifications := brokerapi.NewVerificationService(backend, contracts)
	deploy := brokerapi.NewDeployService(contracts, verifications)

	s := &Server{
		storage:       backend,
		contracts:     contracts,
		verifications: verifications,
		deploy:        deploy,
		logger:        log.Default(),
		mux:           http.NewServeMux(),
	}

	s.setupRoutes()
	return s
}

// WithLogger sets a custom logger for the server.
func (s *Server) WithLogger(logger log.Logger) *Server {
	s.logger = logger
	return s
}

// setupRoutes configures the HTTP routes.
func (s *Server) setupRoutes() {
	// Health check
	s.mux.HandleFunc("GET /health", s.handleHealth)

	// Contract management
	s.mux.HandleFunc("POST /contracts", s.handlePublishContract)
	s.mux.HandleFunc("GET /contracts", s.handleListContracts)
	s.mux.HandleFunc("GET /contracts/{id}", s.handleGetContract)
	s.mux.HandleFunc("POST /contracts/{id}/tags", s.handleTagContract)
	s.mux.HandleFunc("DELETE /contracts/{id}/tags", s.handleUntagContract)
	s.mux.HandleFunc("POST /contracts/{id}/deprecate", s.handleDeprecateContract)

	// Verification
	s.mux.HandleFunc("POST /verifications", s.handleRecordVerification)
	s.mux.HandleFunc("GET /verifications/{contractID}", s.handleListVerifications)
	s.mux.HandleFunc("GET /verifications/{contractID}/{providerVersion}", s.handleGetVerification)

	// Deployment
	s.mux.HandleFunc("GET /can-deploy", s.handleCanDeploy)
	s.mux.HandleFunc("GET /matrix", s.handleMatrix)
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// handleHealth returns a simple health check response.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handlePublishContract handles contract publishing.
func (s *Server) handlePublishContract(w http.ResponseWriter, r *http.Request) {
	var c contract.Contract
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	result, err := s.contracts.Publish(r.Context(), &c)
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusCreated, result)
}

// handleListContracts lists contracts with optional filters.
func (s *Server) handleListContracts(w http.ResponseWriter, r *http.Request) {
	filter := brokerapi.ContractFilter{
		Consumer: r.URL.Query().Get("consumer"),
		Provider: r.URL.Query().Get("provider"),
		Tag:      r.URL.Query().Get("tag"),
		Version:  r.URL.Query().Get("version"),
	}

	if status := r.URL.Query().Get("status"); status != "" {
		validStatus, err := validateStatus(status)
		if err != nil {
			s.errorResponse(w, http.StatusBadRequest, "invalid status: must be draft, published, or deprecated")
			return
		}
		filter.Status = validStatus
	}

	contracts, err := s.contracts.List(r.Context(), &filter)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, contracts)
}

// handleGetContract retrieves a contract by ID.
func (s *Server) handleGetContract(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.errorResponse(w, http.StatusBadRequest, "contract ID is required")
		return
	}

	c, err := s.contracts.GetByID(r.Context(), id)
	if err == storage.ErrNotFound {
		s.errorResponse(w, http.StatusNotFound, "contract not found")
		return
	}
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, c)
}

// handleTagContract adds tags to a contract.
func (s *Server) handleTagContract(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.errorResponse(w, http.StatusBadRequest, "contract ID is required")
		return
	}

	var req struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Tags) > maxTags {
		s.errorResponse(w, http.StatusBadRequest, fmt.Sprintf("too many tags: maximum %d allowed", maxTags))
		return
	}

	if err := s.contracts.Tag(r.Context(), id, req.Tags); err != nil {
		if err == storage.ErrNotFound {
			s.errorResponse(w, http.StatusNotFound, "contract not found")
			return
		}
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleUntagContract removes tags from a contract.
func (s *Server) handleUntagContract(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.errorResponse(w, http.StatusBadRequest, "contract ID is required")
		return
	}

	tagsParam := r.URL.Query().Get("tags")
	if tagsParam == "" {
		s.errorResponse(w, http.StatusBadRequest, "tags parameter is required")
		return
	}

	tags := strings.Split(tagsParam, ",")
	if len(tags) > maxTags {
		s.errorResponse(w, http.StatusBadRequest, fmt.Sprintf("too many tags: maximum %d allowed", maxTags))
		return
	}

	if err := s.contracts.Untag(r.Context(), id, tags); err != nil {
		if err == storage.ErrNotFound {
			s.errorResponse(w, http.StatusNotFound, "contract not found")
			return
		}
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDeprecateContract marks a contract as deprecated.
func (s *Server) handleDeprecateContract(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.errorResponse(w, http.StatusBadRequest, "contract ID is required")
		return
	}

	if err := s.contracts.Deprecate(r.Context(), id); err != nil {
		if err == storage.ErrNotFound {
			s.errorResponse(w, http.StatusNotFound, "contract not found")
			return
		}
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRecordVerification records a verification result.
func (s *Server) handleRecordVerification(w http.ResponseWriter, r *http.Request) {
	var result brokerapi.VerificationResult
	if err := json.NewDecoder(r.Body).Decode(&result); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if err := s.verifications.RecordResult(r.Context(), &result); err != nil {
		s.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// handleListVerifications lists verification results for a contract.
func (s *Server) handleListVerifications(w http.ResponseWriter, r *http.Request) {
	contractID := r.PathValue("contractID")
	if contractID == "" {
		s.errorResponse(w, http.StatusBadRequest, "contract ID is required")
		return
	}

	results, err := s.verifications.ListResults(r.Context(), contractID)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, results)
}

// handleGetVerification retrieves a specific verification result.
func (s *Server) handleGetVerification(w http.ResponseWriter, r *http.Request) {
	contractID := r.PathValue("contractID")
	providerVersion := r.PathValue("providerVersion")

	if contractID == "" || providerVersion == "" {
		s.errorResponse(w, http.StatusBadRequest, "contract ID and provider version are required")
		return
	}

	result, err := s.verifications.GetResult(r.Context(), contractID, providerVersion)
	if err == storage.ErrNotFound {
		s.errorResponse(w, http.StatusNotFound, "verification not found")
		return
	}
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, result)
}

// handleCanDeploy checks if a service can be deployed.
func (s *Server) handleCanDeploy(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	version := r.URL.Query().Get("version")
	env := r.URL.Query().Get("environment")

	if service == "" || version == "" {
		s.errorResponse(w, http.StatusBadRequest, "service and version parameters are required")
		return
	}

	result, err := s.deploy.CanDeploy(r.Context(), service, version, env)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, result)
}

// handleMatrix returns the compatibility matrix.
func (s *Server) handleMatrix(w http.ResponseWriter, r *http.Request) {
	consumer := r.URL.Query().Get("consumer")
	provider := r.URL.Query().Get("provider")

	matrix, err := s.deploy.GetMatrix(r.Context(), consumer, provider)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, matrix)
}

// jsonResponse sends a JSON response with the given status code.
func (s *Server) jsonResponse(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log error but can't do much since headers are already sent
		s.logger.Error("failed to encode JSON response", log.Err(err))
	}
}

// errorResponse sends an error response.
func (s *Server) errorResponse(w http.ResponseWriter, code int, message string) {
	s.jsonResponse(w, code, map[string]string{"error": message})
}

// Config holds server configuration.
type Config struct {
	StorageDir string
	Addr       string
	Logger     log.Logger
}

// Run starts the broker server with the given configuration.
func Run(ctx context.Context, cfg Config) error {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	backend, err := storage.NewFilesystemBackend(cfg.StorageDir)
	if err != nil {
		return fmt.Errorf("failed to create storage backend: %w", err)
	}

	server := NewServer(backend).WithLogger(logger)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("failed to shutdown server", log.Err(err))
		}
	}()

	logger.Info("broker server listening", log.String("addr", cfg.Addr))
	return httpServer.ListenAndServe()
}
