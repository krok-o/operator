package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/krok-o/operator/api/v1alpha1"
	"github.com/krok-o/operator/pkg/providers"
)

type Server struct {
	Address           string
	PlatformProviders map[string]providers.Platform
	Client            client.Client
	Logger            logr.Logger
	Namespace         string
}

func NewServer(addr string, platformProviders map[string]providers.Platform, client client.Client, log logr.Logger, namespace string) *Server {
	return &Server{
		Address:           addr,
		PlatformProviders: platformProviders,
		Client:            client,
		Logger:            log.WithName("handler-logger"),
		Namespace:         namespace,
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

// Payload is used to parse the ref out of the payload.
type Payload struct {
	Ref string `json:"ref"`
}

// Handler handles hook requests made to this operator.
func (s *Server) Handler(w http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	repo, ok := vars["repository"]
	logAndFail := func(status int, err error, message string) {
		w.WriteHeader(http.StatusBadRequest)
		s.Logger.Error(err, message)
		fmt.Fprintf(w, message)
	}
	if !ok {
		logAndFail(http.StatusBadRequest, fmt.Errorf("invalid or missing repository name"), "failed to run handler")
		return
	}
	platform, ok := vars["platform"]
	if !ok {
		logAndFail(http.StatusBadRequest, fmt.Errorf("invalid or missing platform identifier"), "failed to run handler")
		return
	}

	provider, ok := s.PlatformProviders[platform]
	if !ok {
		logAndFail(http.StatusBadRequest, fmt.Errorf("platform %q is not supported", platform), "failed to run handler")
		return
	}

	repository := &v1alpha1.KrokRepository{}
	if err := s.Client.Get(context.Background(), types.NamespacedName{
		Name:      repo,
		Namespace: s.Namespace,
	}, repository); err != nil {
		logAndFail(http.StatusBadRequest, err, "failed to find associated repo object")
		return
	}

	auth := &corev1.Secret{}
	if err := s.Client.Get(context.Background(), types.NamespacedName{
		Name:      repository.Spec.AuthSecretRef.Name,
		Namespace: repository.Spec.AuthSecretRef.Namespace,
	}, auth); err != nil {
		logAndFail(http.StatusBadRequest, err, "failed to find associated provider object")
		return
	}

	secret, ok := auth.Data["secret"]
	if !ok {
		logAndFail(http.StatusBadRequest, fmt.Errorf("failed to find 'secret' in auth secret data"), "failed to run handler")
		return
	}

	// Validation closes the request Body, but we need to content later on.
	buf, _ := io.ReadAll(request.Body)
	rdr1 := io.NopCloser(bytes.NewBuffer(buf))
	rdr2 := io.NopCloser(bytes.NewBuffer(buf))
	request.Body = rdr1
	ping, err := provider.ValidateRequest(context.Background(), request, string(secret))
	if err != nil {
		logAndFail(http.StatusBadRequest, err, "failed to validate request")
		return
	}

	if ping {
		w.WriteHeader(http.StatusOK)
		return
	}

	content, err := io.ReadAll(rdr2)
	if err != nil {
		logAndFail(http.StatusInternalServerError, err, "failed to read request body for payload")
		return
	}

	// Get the REF from the payload.
	payload := &Payload{}
	if err := json.Unmarshal(content, payload); err != nil {
		logAndFail(http.StatusInternalServerError, err, "failed to parse payload and get ref")
		return
	}

	// This should only happen if it wasn't a `Ping` event which is the initial registration.
	eventType, err := provider.GetEventType(context.Background(), request)
	if err != nil {
		logAndFail(http.StatusBadRequest, err, "failed to get event type")
		return
	}
	event := &v1alpha1.KrokEvent{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KrokEvent",
			APIVersion: "delivery.krok.app/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.generateEventName(repository.Name),
			Namespace: repository.Namespace,
		},
		Spec: v1alpha1.KrokEventSpec{
			Payload:   string(content),
			Type:      eventType,
			CommitRef: payload.Ref,
		},
	}
	// Set external object ControllerReference to the provider ref.
	if err := controllerutil.SetControllerReference(repository, event, s.Client.Scheme()); err != nil {
		logAndFail(http.StatusInternalServerError, err, "failed to set owner reference for created event")
		return
	}

	if err := s.Client.Create(context.Background(), event); err != nil {
		logAndFail(http.StatusInternalServerError, err, "failed to create Event object")
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) generateEventName(repoName string) string {
	return fmt.Sprintf("%s-event-%d", repoName, time.Now().Unix())
}
