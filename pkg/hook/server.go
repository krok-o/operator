package hook

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/krok-o/operator/pkg/providers"
)

// TODO: This needs a client.
type Server struct {
	Address           string
	PlatformProviders map[string]providers.Platform
}

func NewServer(addr string, platformProviders map[string]providers.Platform) *Server {
	return &Server{
		Address:           addr,
		PlatformProviders: platformProviders,
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
	//repo, ok := vars["repository"]
	//if !ok {
	//	w.WriteHeader(http.StatusBadRequest)
	//	fmt.Fprintf(w, "invalid or missing repository name")
	//	return
	//}
	platform, ok := vars["platform"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid or missing platform identifier")
		return
	}

	provider, ok := s.PlatformProviders[platform]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, fmt.Sprintf("platform %q is not supported", platform))
		return
	}

	// TODO: Needs to find the token secret.
	if err := provider.ValidateRequest(context.Background(), request, ""); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, fmt.Sprintf("failed to validate request: %s", err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
}
