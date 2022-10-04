package hook

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/krok-o/operator/api/v1alpha1"
	"github.com/krok-o/operator/pkg/providers"
)

type Server struct {
	Address           string
	PlatformProviders map[string]providers.Platform
	Client            client.Client
	Logger            logr.Logger
}

func NewServer(addr string, platformProviders map[string]providers.Platform, client client.Client, log logr.Logger) *Server {
	return &Server{
		Address:           addr,
		PlatformProviders: platformProviders,
		Client:            client,
		Logger:            log.WithName("handler-logger"),
	}
}

func (s *Server) ListenAndServe() error {
	r := mux.NewRouter()
	r.HandleFunc("/hooks/{repository}/{platform}/callback", s.Handler)
	srv := &http.Server{
		Handler:      r,
		Addr:         s.Address,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	return srv.ListenAndServe()
}

// Handler handles hook requests made to this operator.
func (s *Server) Handler(w http.ResponseWriter, request *http.Request) {

	vars := mux.Vars(request)
	repo, ok := vars["repository"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		s.Logger.Error(fmt.Errorf("invalid or missing repository name"), "failed to run handler")
		fmt.Fprintf(w, "invalid or missing repository name")
		return
	}
	platform, ok := vars["platform"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		s.Logger.Error(fmt.Errorf("invalid or missing platform identifier"), "failed to run handler")
		fmt.Fprintf(w, "invalid or missing platform identifier")
		return
	}

	provider, ok := s.PlatformProviders[platform]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		s.Logger.Error(fmt.Errorf("platform %q is not supported", platform), "failed to run handler")
		fmt.Fprintf(w, fmt.Sprintf("platform %q is not supported", platform))
		return
	}

	repository := &v1alpha1.KrokRepository{}
	if err := s.Client.Get(context.Background(), types.NamespacedName{
		Name:      repo,
		Namespace: "krok-system",
	}, repository); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.Logger.Error(err, "failed to find associated provider object")
		fmt.Fprintf(w, fmt.Sprintf("failed to find associated provider object: %v", err))
		return
	}

	auth := &corev1.Secret{}
	if err := s.Client.Get(context.Background(), types.NamespacedName{
		Name:      repository.Spec.AuthSecretRef.Name,
		Namespace: repository.Spec.AuthSecretRef.Namespace,
	}, auth); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.Logger.Error(err, "failed to find associated provider object")
		fmt.Fprintf(w, fmt.Sprintf("failed to find associated provider object: %v", err))
		return
	}

	secret, ok := auth.Data["secret"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		s.Logger.Error(fmt.Errorf("failed to find 'secret' in auth secret data"), "failed to run handler")
		fmt.Fprintf(w, "failed to find 'secret' in auth secret data")
		return
	}

	if err := provider.ValidateRequest(context.Background(), request, string(secret)); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.Logger.Error(err, "failed to validate request")
		fmt.Fprintf(w, fmt.Sprintf("failed to validate request: %s", err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
}
