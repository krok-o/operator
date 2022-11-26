package hook

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/krok-o/operator/api/v1alpha1"
	"github.com/krok-o/operator/pkg/providers"
)

var jobFinalizer = "event.krok.app/finalizer"

type Server struct {
	Address           string
	PlatformProviders map[string]providers.Platform
	Client            client.Client
	Logger            logr.Logger
	Namespace         string
	Clock             providers.Clock
}

func NewServer(addr string, platformProviders map[string]providers.Platform, client client.Client, log logr.Logger, namespace string, clock providers.Clock) *Server {
	return &Server{
		Address:           addr,
		PlatformProviders: platformProviders,
		Client:            client,
		Logger:            log.WithName("handler-logger"),
		Namespace:         namespace,
		Clock:             clock,
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
		logAndFail(http.StatusBadRequest, fmt.Errorf("invalid or missing repository name"), "invalid or missing repository name")
		return
	}
	platform, ok := vars["platform"]
	if !ok {
		logAndFail(http.StatusBadRequest, fmt.Errorf("invalid or missing platform identifier"), "invalid or missing platform identifier")
		return
	}

	provider, ok := s.PlatformProviders[platform]
	if !ok {
		logAndFail(http.StatusBadRequest, fmt.Errorf("platform %q is not supported", platform), fmt.Sprintf("platform %q is not supported", platform))
		return
	}

	repository := &v1alpha1.KrokRepository{}
	ctx := context.Background()
	if err := s.Client.Get(ctx, types.NamespacedName{
		Name:      repo,
		Namespace: s.Namespace,
	}, repository); err != nil {
		logAndFail(http.StatusBadRequest, err, "failed to find associated repo object")
		return
	}

	auth := &corev1.Secret{}
	if err := s.Client.Get(ctx, types.NamespacedName{
		Name:      repository.Spec.AuthSecretRef.Name,
		Namespace: repository.Spec.AuthSecretRef.Namespace,
	}, auth); err != nil {
		logAndFail(http.StatusBadRequest, err, "failed to find associated auth secret")
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
	ping, err := provider.ValidateRequest(ctx, request, string(secret))
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

	// This should only happen if it wasn't a `Ping` event which is the initial registration.
	eventType, err := provider.GetEventType(ctx, request)
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
			Payload: string(content),
			Type:    eventType,
		},
	}

	supportsPlatform := func(platforms []string) bool {
		for _, p := range platforms {
			if p == repository.Spec.Platform {
				return true
			}
		}

		return false
	}
	for _, commandRef := range repository.Spec.Commands {
		command := &v1alpha1.KrokCommand{}
		if err := s.Client.Get(ctx, types.NamespacedName{
			Namespace: commandRef.Namespace,
			Name:      commandRef.Name,
		}, command); err != nil {
			logAndFail(http.StatusBadRequest, fmt.Errorf("failed to get command object: %w", err), "failed to get command object")
			return
		}
		if !command.Spec.Enabled {
			continue
		}
		if !supportsPlatform(command.Spec.Platforms) {
			s.Logger.Info(
				"skipped command as it does not support the given platform",
				"command",
				klog.KObj(command),
				"platform",
				repository.Spec.Platform,
				"supported-platforms",
				command.Spec.Platforms,
			)
			continue
		}
		event.Spec.CommandsToRun = append(event.Spec.CommandsToRun, v1alpha1.CommandTemplate{
			Name: command.Name,
			Spec: command.Spec,
		})
	}

	controllerutil.AddFinalizer(event, jobFinalizer)
	// Set external object ControllerReference to the provider ref.
	if err := controllerutil.SetControllerReference(repository, event, s.Client.Scheme()); err != nil {
		logAndFail(http.StatusInternalServerError, err, "failed to set owner reference for created event")
		return
	}

	if err := s.Client.Create(ctx, event); err != nil {
		logAndFail(http.StatusInternalServerError, err, "failed to create Event object")
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) generateEventName(repoName string) string {
	return fmt.Sprintf("%s-event-%d", repoName, s.Clock.Now().Unix())
}
