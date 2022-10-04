package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	ggithub "github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"gopkg.in/go-playground/webhooks.v5/github"
	"k8s.io/klog/v2"

	"github.com/krok-o/operator/api/v1alpha1"
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
	hook, resp, err := githubClient.Repositories.CreateHook(context.Background(), repoUser, repoName, &ggithub.Hook{
		Events: repo.Spec.Events,
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