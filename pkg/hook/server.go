package hook

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type Server struct {
	Address string
}

func NewServer(addr string) *Server {
	return &Server{
		Address: addr,
	}
}

func (s *Server) ListenAndServe() error {
	r := mux.NewRouter()
	r.HandleFunc("/hooks/{repository}/{platform}/callback", Handler)
	//http.Handle("/", r)
	srv := &http.Server{
		Handler:      r,
		Addr:         s.Address,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	return srv.ListenAndServe()
}

// Handler handles hook requests made to this operator.
func Handler(w http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	repo, ok := vars["repository"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid or missing repository name")
		return
	}
	platform := vars["platform"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid or missing platform identifier")
		return
	}
	fmt.Fprintf(w, "received: repo: %s, platform: %s", repo, platform)
}
