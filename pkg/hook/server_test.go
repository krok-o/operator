package hook

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/krok-o/operator/api/v1alpha1"
	"github.com/krok-o/operator/pkg/providers"
	"github.com/krok-o/operator/pkg/providers/providersfakes"
)

// FakeResponseWriter implements http.ResponseWriter,
// it is used for testing purpose only
type FakeResponseWriter struct {
	content []byte
	code    int
}

func (fw *FakeResponseWriter) Header() http.Header { return http.Header{} }
func (fw *FakeResponseWriter) WriteHeader(code int) {
	fw.code = code
}
func (fw *FakeResponseWriter) Write(bs []byte) (int, error) {
	for _, b := range bs {
		fw.content = append(fw.content, b)
	}
	return len(bs), nil
}

type FakeClock struct{}

func (f *FakeClock) Now() time.Time {
	return time.Date(2022, 1, 1, 1, 1, 1, 0, time.UTC)
}

func TestHandler(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	assert.NoError(t, err)
	err = corev1.AddToScheme(scheme)
	assert.NoError(t, err)
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&v1alpha1.KrokRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KrokRepository",
			APIVersion: v1alpha1.GroupVersion.Group,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "krok-test-repo",
			Namespace: "krok-system",
		},
		Spec: v1alpha1.KrokRepositorySpec{
			URL:      "https://github.com/Skarlso/test",
			Platform: "github",
			AuthSecretRef: v1alpha1.Ref{
				Name:      "auth-secret",
				Namespace: "krok-system",
			},
			ProviderTokenSecretRef: v1alpha1.Ref{
				Name:      "token-secret",
				Namespace: "krok-system",
			},
		},
		Status: v1alpha1.KrokRepositoryStatus{
			UniqueURL: "http://localhost:9999/hook/krok-test-repo/github/callback",
		},
	}, &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: corev1.GroupName,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "auth-secret",
			Namespace: "krok-system",
		},
		Immutable: nil,
		Data: map[string][]byte{
			"secret": []byte("secret"),
		},
		Type: "opaque",
	}).Build()

	fakePlatform := &providersfakes.FakePlatform{}
	fakePlatform.GetEventTypeReturns("push", nil)
	server := NewServer(":9999", map[string]providers.Platform{
		"github": fakePlatform,
	}, k8sClient, logr.Discard(), "krok-system", &FakeClock{})

	fw := &FakeResponseWriter{}

	payload, err := os.ReadFile(filepath.Join("testdata", "push_payload.json"))
	assert.NoError(t, err)
	buffer := bytes.NewBuffer(payload)
	r, err := http.NewRequest(http.MethodPost, "http://localhost:8080", buffer)
	assert.NoError(t, err)

	r = mux.SetURLVars(r, map[string]string{
		"repository": "krok-test-repo",
		"platform":   "github",
	})

	server.Handler(fw, r)
	assert.Equal(t, http.StatusOK, fw.code)
	event := &v1alpha1.KrokEvent{}
	err = k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: "krok-system",
		Name:      "krok-test-repo-event-1640998861",
	}, event)
	assert.NoError(t, err)
	assert.Equal(t, "krok-test-repo-event-1640998861", event.Name)
	assert.Equal(t, "push", event.Spec.Type)
	assert.Equal(t, string(payload), event.Spec.Payload)
}
