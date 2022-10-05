package source_controller

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"github.com/krok-o/operator/api/v1alpha1"
	"github.com/krok-o/operator/pkg/providers"
)

type Server struct {
	Addr     string
	Location string
	Logger   logr.Logger
}

func NewServer(logger logr.Logger, addr string, location string) *Server {
	return &Server{
		Addr:     addr,
		Location: location,
		Logger:   logger.WithName("source-controller"),
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
	location, err := platform.CheckoutCode(context.Background(), event, repository)
	if err != nil {
		return "", fmt.Errorf("failed to checkout code: %w", err)
	}
	// move file under serving location
	filename := filepath.Base(location)
	s.Logger.V(4).Info("preparing to serve file with name", "name", filename)
	newLocation := filepath.Join(s.Location, repository.Name, event.Name, "latest.zip")
	if err := os.MkdirAll(filepath.Dir(newLocation), 0777); err != nil {
		return "", fmt.Errorf("failed to create location '%s': %w", newLocation, err)
	}
	if err := os.Rename(location, newLocation); err != nil {
		return "", fmt.Errorf("failed to move file to its new location: %w", err)
	}
	hostname, err := s.determineStorageAddr()
	if err != nil {
		return "", fmt.Errorf("failed to determine storage address: %w", err)
	}
	format := "http://%s/%s"
	if strings.HasPrefix(hostname, "http://") || strings.HasPrefix(hostname, "https://") {
		format = "%s/%s"
	}
	url := fmt.Sprintf(format, hostname, strings.TrimLeft(newLocation, "/"))
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
