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
	r.HandleFunc("/hooks/{rid:[0-9]+}/{vid:[0-9]+}/callback", HookHandler)
	http.Handle("/", r)
	srv := &http.Server{
		Handler:      r,
		Addr:         s.Address,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	return srv.ListenAndServe()
}

// HookHandler handles hook requests made to this operator.
func HookHandler(w http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	rid, ok := vars["rid"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid or missing repository ID")
		return
	}
	vid := vars["vid"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid or missing vcs ID")
		return
	}
	fmt.Fprintf(w, "received: rid: %s, vid: %s", rid, vid)
}
