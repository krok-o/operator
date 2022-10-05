package hook

import (
	"bytes"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"github.com/krok-o/operator/pkg/providers"
	"github.com/krok-o/operator/pkg/providers/providersfakes"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
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

func TestHandler(t *testing.T) {
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}
	cfg, err := testEnv.Start()
	assert.NoError(t, err)

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	assert.NoError(t, err)

	fakePlatform := &providersfakes.FakePlatform{}

	server := NewServer(":9999", map[string]providers.Platform{
		"github": fakePlatform,
	}, k8sClient, logr.Discard(), "krok-system")

	fw := &FakeResponseWriter{}

	var buf []byte
	buffer := bytes.NewBuffer(buf)
	r, err := http.NewRequest(http.MethodPost, "http://localhost:8080", buffer)
	assert.NoError(t, err)

	r = mux.SetURLVars(r, map[string]string{
		"repository": "krok-test-repo",
		"platform":   "github",
	})
	//ctx := context.WithValue(r.Context(), 0, )

	server.Handler(fw, r)
	fmt.Println("content: ", string(fw.content))

	assert.Equal(t, http.StatusOK, fw.code)
}
