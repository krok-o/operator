package github

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-logr/logr"
	ggithub "github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"gopkg.in/go-playground/webhooks.v5/github"
	"k8s.io/klog/v2"

	"github.com/krok-o/operator/api/v1alpha1"
	"github.com/krok-o/operator/pkg/providers"
)

// Platform is a GitHub based platform implementation.
type Platform struct {
	Logger logr.Logger

	// Used for testing the CreateHook call. There probably is a better way to do this...
	repoMock GoogleGithubRepoService
}

// NewGithubPlatformProvider creates a new hook platform provider for GitHub.
func NewGithubPlatformProvider(log logr.Logger) *Platform {
	return &Platform{
		Logger: log.WithName("platform-logger"),
	}
}

var _ providers.Platform = &Platform{}

// ValidateRequest will take a hook and verify it being a valid hook request according to
// GitHub's rules.
func (g *Platform) ValidateRequest(ctx context.Context, req *http.Request, secret string) (bool, error) {
	req.Header.Set("Content-type", "application/json")

	hook, err := github.New(github.Options.Secret(secret))
	if err != nil {
		return false, fmt.Errorf("failed to create github client: %w", err)
	}
	h, err := hook.Parse(req,
		github.CheckRunEvent,
		github.CheckSuiteEvent,
		github.CommitCommentEvent,
		github.CreateEvent,
		github.DeleteEvent,
		github.DeploymentEvent,
		github.DeploymentStatusEvent,
		github.ForkEvent,
		github.GollumEvent,
		github.InstallationEvent,
		github.InstallationRepositoriesEvent,
		github.IntegrationInstallationRepositoriesEvent,
		github.IssueCommentEvent,
		github.IssuesEvent,
		github.LabelEvent,
		github.MemberEvent,
		github.MembershipEvent,
		github.MetaEvent,
		github.MilestoneEvent,
		github.OrgBlockEvent,
		github.OrganizationEvent,
		github.PageBuildEvent,
		github.PingEvent,
		github.ProjectCardEvent,
		github.ProjectColumnEvent,
		github.ProjectEvent,
		github.PublicEvent,
		github.PullRequestEvent,
		github.PullRequestReviewCommentEvent,
		github.PullRequestReviewEvent,
		github.PushEvent,
		github.ReleaseEvent,
		github.RepositoryEvent,
		github.RepositoryVulnerabilityAlertEvent,
		github.SecurityAdvisoryEvent,
		github.StatusEvent)
	if err != nil {
		g.Logger.Error(err, "Failed to parse github event.")
		return false, err
	}
	switch h.(type) {
	case github.PingPayload:
		g.Logger.V(4).Info("All good, send back ping.")
		return true, nil
	}
	return false, nil
}

// GoogleGithubRepoService is an interface defining the Wrapper Interface
// needed to test the GitHub client.
type GoogleGithubRepoService interface {
	CreateHook(ctx context.Context, owner, repo string, hook *ggithub.Hook) (*ggithub.Hook, *ggithub.Response, error)
}

// GoogleGithubClient is a client that has the ability to replace the actual
// git client.
type GoogleGithubClient struct {
	Repositories GoogleGithubRepoService
	*ggithub.Client
}

// NewGoogleGithubClient creates a wrapper around the GitHub client.
func NewGoogleGithubClient(httpClient *http.Client, repoMock GoogleGithubRepoService) GoogleGithubClient {
	if repoMock != nil {
		return GoogleGithubClient{
			Repositories: repoMock,
		}
	}
	githubClient := ggithub.NewClient(httpClient)

	return GoogleGithubClient{
		Repositories: githubClient.Repositories,
	}
}

// GetEventID Based on the platform, retrieve the ID of the event.
func (g *Platform) GetEventID(ctx context.Context, r *http.Request) (string, error) {
	id := r.Header.Get("X-GitHub-Delivery")
	if id == "" {
		return "", errors.New("event id not found for request")
	}
	return id, nil
}

// GetEventType Based on the platform, retrieve the Type of the event.
func (g *Platform) GetEventType(ctx context.Context, r *http.Request) (string, error) {
	event := r.Header.Get("X-Github-Event")
	if len(event) == 0 {
		return "", fmt.Errorf("failed to get event type")
	}
	return event, nil
}

// CreateHook can create a hook for the GitHub platform.
func (g *Platform) CreateHook(ctx context.Context, repo *v1alpha1.KrokRepository, platformToken, secret string) error {
	log := g.Logger.WithValues("uniqueURL", repo.Status.UniqueURL, "repository", klog.KObj(repo))
	if len(repo.Spec.Events) == 0 {
		return errors.New("no events provided to subscribe to")
	}
	if repo.Status.UniqueURL == "" {
		return errors.New("unique callback url is empty")
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: platformToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	config := make(map[string]interface{})
	config["url"] = repo.Status.UniqueURL
	config["secret"] = secret
	config["content_type"] = "json"

	// figure out a way to mock this nicely later on.
	githubClient := NewGoogleGithubClient(tc, g.repoMock)
	repoName := path.Base(repo.Spec.URL)
	repoName = strings.TrimSuffix(repoName, ".git")
	// var repoLocation string
	re := regexp.MustCompile("^(https|git)(://|@)([^/:]+)[/:]([^/:]+)/(.+)$")
	m := re.FindAllStringSubmatch(repo.Spec.URL, -1)
	if m == nil {
		return errors.New("failed to extract url parameters from git url")
	}
	if len(m[0]) < 5 {
		return errors.New("failed to extract repo user from the url")
	}
	repoUser := m[0][4]
	events, ok := repo.Spec.Events[v1alpha1.GITHUB]
	if !ok {
		return errors.New("no events found for provider 'github'")
	}
	hook, resp, err := githubClient.Repositories.CreateHook(context.Background(), repoUser, repoName, &ggithub.Hook{
		Events: events,
		Name:   ggithub.String("web"),
		Active: ggithub.Bool(true),
		Config: config,
	})
	if err != nil {
		if resp.StatusCode == 422 && strings.Contains(err.Error(), "") {
			log.V(4).Info("hook already exists")
			return nil
		}
		return fmt.Errorf("failed to create hook: %w", err)
	}
	if resp.StatusCode < 200 && resp.StatusCode > 299 {
		return fmt.Errorf("invalid status code %d received from hook creation", resp.StatusCode)
	}
	log.V(4).WithValues("name", *hook.Name).Info("Hook with name successfully created.")
	return nil
}

// GetRefIfPresent returns a Ref if the payload contains one.
func (g *Platform) GetRefIfPresent(ctx context.Context, event *v1alpha1.KrokEvent) (string, string, error) {
	var (
		ref    string
		branch string
	)
	switch event.Spec.Type {
	case string(github.PullRequestEvent):
		payload := &github.PullRequestPayload{}
		if err := json.Unmarshal([]byte(event.Spec.Payload), payload); err != nil {
			return "", "", fmt.Errorf("failed to unmarshall payload: %w", err)
		}
		ref = fmt.Sprintf("pull/%d/head:%s", payload.Number, payload.PullRequest.Base.Ref)
		branch = payload.PullRequest.Base.Ref
	case string(github.PushEvent):
		payload := &github.PushPayload{}
		if err := json.Unmarshal([]byte(event.Spec.Payload), payload); err != nil {
			return "", "", fmt.Errorf("failed to unmarshall payload: %w", err)
		}
		ref = payload.Ref
		split := strings.Split(ref, "/")
		branch = split[len(split)-1]
	default:
		return "not-found", "", nil
	}
	return ref, branch, nil
}

// CheckoutCode will get the code given an event which needs the codebase.
// It will create a ZIP file from the source code which is offered by a server running a file-server with given URLs for
// each artifact.
func (g *Platform) CheckoutCode(ctx context.Context, event *v1alpha1.KrokEvent, repository *v1alpha1.KrokRepository, location string) (string, error) {
	ref, branch, err := g.GetRefIfPresent(ctx, event)
	if err != nil {
		return "", fmt.Errorf("failed to get ref: %w", err)
	}
	if ref == "not-found" {
		// No ref, no need to check out the code.
		return "", nil
	}

	// TODO: Find the right location for this
	dir, err := os.MkdirTemp("", "clone")
	if err != nil {
		return "", fmt.Errorf("failed to initialise temp folder: %w", err)
	}
	fs := osfs.New(dir)
	r, err := git.Init(filesystem.NewStorage(fs, cache.NewObjectLRUDefault()), nil)
	if err != nil {
		return "", fmt.Errorf("failed to initialise git repo: %w", err)
	}
	if _, err = r.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{repository.Spec.URL},
	}); err != nil {
		return "", fmt.Errorf("failed to create remote: %w", err)
	}
	if err := r.Fetch(&git.FetchOptions{
		RefSpecs: []config.RefSpec{config.RefSpec(ref + ":" + branch)},
		Depth:    1,
	}); err != nil {
		return "", fmt.Errorf("failed to fetch remote ref '%s': %w", ref, err)
	}
	commitRef, err := r.Reference(plumbing.ReferenceName(branch), true)
	if err != nil {
		return "", fmt.Errorf("failed to find reference for branch '%s': %w", branch, err)
	}
	cc, err := r.CommitObject(commitRef.Hash())
	if err != nil {
		return "", fmt.Errorf("failed to get commit object: %w", err)
	}
	tree, err := r.TreeObject(cc.TreeHash)
	if err != nil {
		return "", fmt.Errorf("failed to create TreeObject: %w", err)
	}
	newLocation := filepath.Join(location, repository.Name, event.Name, branch+".zip")
	if err := os.MkdirAll(filepath.Dir(newLocation), 0777); err != nil {
		return "", fmt.Errorf("failed to create location '%s': %w", newLocation, err)
	}
	zipFile, err := os.Create(newLocation)
	if err != nil {
		return "", fmt.Errorf("failed to create zipfile: %w", err)
	}
	defer zipFile.Close()
	z := zip.NewWriter(zipFile)
	defer z.Close()
	addFile := func(f *object.File) error {
		fw, err := z.Create(f.Name)
		if err != nil {
			return err
		}

		fr, err := f.Reader()
		if err != nil {
			return err
		}

		_, err = io.Copy(fw, fr)
		if err != nil {
			return err
		}

		return fr.Close()
	}

	if err := tree.Files().ForEach(addFile); err != nil {
		return "", fmt.Errorf("failed to traverse tree object: %w", err)
	}

	// The calling source_controller will take this file and place it under
	// /data/{repository}/{event}/{branch}.zip
	// And then, the URL for that will be http://{service}/data/{repository}/{event}/{branch}.zip
	return newLocation, nil
}
