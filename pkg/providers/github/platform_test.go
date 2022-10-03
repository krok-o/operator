package github

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-github/github"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/krok-o/operator/api/v1alpha1"
)

type mockGithubRepositoryService struct {
	Hook     *github.Hook
	Response *github.Response
	Error    error
	Owner    string
	Repo     string
}

func (mgc *mockGithubRepositoryService) CreateHook(ctx context.Context, owner, repo string, hook *github.Hook) (*github.Hook, *github.Response, error) {
	if owner != mgc.Owner {
		return nil, nil, errors.New("owner did not equal expected owner: was: " + owner)
	}
	if repo != mgc.Repo {
		return nil, nil, errors.New("repo did not equal expected repo: was: " + repo)
	}
	return mgc.Hook, mgc.Response, mgc.Error
}

func TestGithub_CreateHook(t *testing.T) {
	npp := NewGithubPlatformProvider(logr.Discard())
	mock := &mockGithubRepositoryService{}
	mock.Hook = &github.Hook{
		Name: github.String("test hook"),
		URL:  github.String("https://api.github.com/repos/krok-o/krok/hooks/44321286/test"),
	}
	mock.Response = &github.Response{
		Response: &http.Response{
			Status: "Ok",
		},
	}
	mock.Owner = "krok-o"
	mock.Repo = "krok"
	npp.repoMock = mock
	err := npp.CreateHook(context.Background(), &v1alpha1.KrokRepository{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-repository",
		},
		Spec: v1alpha1.KrokRepositorySpec{
			URL:      "https://github.com/krok-o/krok",
			Platform: v1alpha1.GITHUB,
			Events:   []string{"push"},
		},
		Status: v1alpha1.KrokRepositoryStatus{
			UniqueURL: "https://krok.com/hooks/0/0/callback",
		},
	}, "token", "secret")
	assert.NoError(t, err)
}

func TestGithub_GetEventID(t *testing.T) {
	npp := NewGithubPlatformProvider(logr.Discard())
	header := http.Header{}
	header.Add("X-GitHub-Delivery", "ID")
	id, err := npp.GetEventID(context.Background(), &http.Request{
		Header: header,
	})
	assert.NoError(t, err)
	assert.Equal(t, "ID", id)

	_, err = npp.GetEventID(context.Background(), &http.Request{})
	assert.Errorf(t, err, "event id not found for request")
}
