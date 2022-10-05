package source_controller

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"github.com/krok-o/operator/api/v1alpha1"
	"github.com/krok-o/operator/pkg/providers"
)

type Server struct {
	Addr         string
	ArtifactBase string
	Location     string
	Logger       logr.Logger
}

func NewServer(logger logr.Logger, addr string, location string, artifactBase string) *Server {
	return &Server{
		Addr:         addr,
		ArtifactBase: artifactBase,
		Location:     location,
		Logger:       logger.WithName("source-controller"),
	}
}

func (s *Server) ServeStaticFiles() error {
	r := mux.NewRouter()

	r.PathPrefix("/serve/").Handler(http.StripPrefix("/serve/", http.FileServer(http.Dir(s.Location))))

	srv := &http.Server{
		Handler:      r,
		Addr:         s.Addr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	return srv.ListenAndServe()
}

func (s *Server) FetchCode(platform providers.Platform, event *v1alpha1.KrokEvent, repository *v1alpha1.KrokRepository) (string, error) {
	location, err := platform.CheckoutCode(context.Background(), event, repository, s.Location)
	if err != nil {
		return "", fmt.Errorf("failed to checkout code: %w", err)
	}
	// No need for checkout logic.
	if location == "" {
		return "", nil
	}
	if s.ArtifactBase == "" {
		addr, err := s.determineStorageAddr()
		if err != nil {
			return "", fmt.Errorf("failed to determine hostname: %w", err)
		}
		s.ArtifactBase = addr
	}
	format := "http://%s/%s"
	if strings.HasPrefix(s.ArtifactBase, "http://") || strings.HasPrefix(s.ArtifactBase, "https://") {
		format = "%s/%s"
	}

	url := fmt.Sprintf(format, s.ArtifactBase, strings.TrimLeft(location, "/"))
	return url, nil
}

func (s *Server) determineStorageAddr() (string, error) {
	host, port, err := net.SplitHostPort(s.Addr)
	if err != nil {
		s.Logger.Error(err, "unable to parse storage address")
		return "", fmt.Errorf("failed to determine storage address: %w", err)
	}
	switch host {
	case "":
		host = "localhost"
	case "0.0.0.0":
		host = os.Getenv("HOSTNAME")
		if host == "" {
			hn, err := os.Hostname()
			if err != nil {
				s.Logger.Error(err, "0.0.0.0 specified in storage addr but hostname is invalid")
				return "", fmt.Errorf("failed to retrieve hostname: %w", err)
			}
			host = hn
		}
	}
	return net.JoinHostPort(host, port), nil
}
