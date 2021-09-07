package cmd

import (
	"context"
	"fmt"
	gitconfig "github.com/go-git/go-git/v5/config"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RossyWhite/flux-helm-version-updater/internal/helm"
	"github.com/RossyWhite/flux-helm-version-updater/internal/setter"
	helmv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v38/github"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"
	"golang.org/x/xerrors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// helmVersionUpdater is a main struct which controls update processes
type helmVersionUpdater struct {
	client.Client
	gitRoot string
	repo    *git.Repository
	wt      *git.Worktree
	head    *plumbing.Reference
	schema  *runtime.Scheme
	conf    *config
}

// config holds the arguments given by users
type config struct {
	GithubToken string `mapstructure:"github-token"`
	Name        string `mapstructure:"git-name"`
	Email       string `mapstructure:"git-email"`
	Target      string `mapstructure:"target"`
	Path        string `mapstructure:"path"`
	Prefix      string `mapstructure:"prefix"`
}

// initScheme initializes a new scheme, with related CRDs
func initScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(helmv2.AddToScheme(s))
	utilruntime.Must(sourcev1.AddToScheme(s))
	return s
}

// initConf parses command line args and create *config with them
func initConf() *config {
	pflag.String("github-token", "", "Access token of GitHub")
	pflag.String("git-name", "", "Name of the git user")
	pflag.String("git-email", "", "Name address of the git user")
	pflag.String("target", "", "Target repository name in http format")
	pflag.String("path", "", "Relative path of the repository to check update")
	pflag.String("prefix", "", "Prefix which will attach to branch name")
	pflag.Parse()
	_ = viper.BindPFlags(pflag.CommandLine)

	_ = viper.BindEnv("github-token")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	var conf config
	if err := viper.Unmarshal(&conf); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return &conf
}

// NewHelmVersionUpdateCmd returns *helmVersionUpdater with some setups
func NewHelmVersionUpdateCmd() *helmVersionUpdater {
	updater := &helmVersionUpdater{}
	updater.schema = initScheme()
	updater.conf = initConf()

	k8s, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: updater.schema})
	if err != nil {
		log.Fatalf("can't initialize kubernetes client: %v", err)
	}
	updater.Client = k8s

	return updater
}

// Execute executes HelmRelease update to the target repository.
func (r *helmVersionUpdater) Execute() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := r.cloneTargetRepository(); err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(r.gitRoot) }()

	var hrs helmv2.HelmReleaseList
	if err := r.List(ctx, &hrs, &client.ListOptions{}); err != nil {
		return err
	}

	for _, hr := range hrs.Items {
		latest, err := r.getLatestChartVer(ctx, &hr)
		if err != nil {
			log.Println(err)
			continue
		}

		if err := r.createVersionUpdatePR(ctx, &hr, latest); err != nil {
			log.Println(err)
			continue
		}
	}

	return nil
}

// cloneTargetRepository clones the remote repository into TmpDir
func (r *helmVersionUpdater) cloneTargetRepository() error {
	path, err := os.MkdirTemp("", "")
	if err != nil {
		return xerrors.Errorf("failed to create tmpdir: %+w", err)
	}

	repo, err := git.PlainClone(path, false, &git.CloneOptions{
		Auth:     &http.BasicAuth{Username: r.conf.GithubToken},
		URL:      r.conf.Target,
		Progress: io.Discard,
		Depth:    1,
	})

	if err != nil {
		return xerrors.Errorf("failed to clone: %+w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return xerrors.Errorf("failed to get head: %+w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return xerrors.Errorf("failed to get worktree: %+w", err)
	}

	r.gitRoot = path
	r.repo = repo
	r.head = head
	r.wt = wt

	return nil
}

// getLatestChartVer fetch the latest chart version of the given HelmRelease.
func (r *helmVersionUpdater) getLatestChartVer(ctx context.Context, hr *helmv2.HelmRelease) (string, error) {
	var repo sourcev1.HelmRepository
	repoName := types.NamespacedName{
		Namespace: hr.Spec.Chart.Spec.SourceRef.Namespace,
		Name:      hr.Spec.Chart.Spec.SourceRef.Name,
	}

	if err := r.Get(ctx, repoName, &repo); err != nil {
		return "", xerrors.Errorf("failed to get: %+w", err)
	}

	chart, err := helm.NewChartRepository(repo.Spec.URL)
	if err != nil {
		return "", xerrors.Errorf("failed init chart repo: %+w", err)
	}

	ver, err := chart.FindLatestVersion(hr.Name, hr.Spec.Chart.Spec.Version)
	if err != nil {
		return "", xerrors.Errorf("failed to find latest version: %+w", err)
	}

	return ver, nil
}

// createVersionUpdatePR create a pull request which replaces the version tag with the latest version.
func (r *helmVersionUpdater) createVersionUpdatePR(ctx context.Context, hr *helmv2.HelmRelease, v string) error {
	branchName := fmt.Sprintf("helmupdate-%s-%s", hr.Namespace, hr.Name)
	if r.conf.Prefix != "" {
		branchName = fmt.Sprintf("%s-%s", r.conf.Prefix, branchName)
	}

	if err := r.checkout(branchName); err != nil {
		return err
	}

	path := r.gitRoot
	if r.conf.Path != "" {
		path = filepath.Join(r.gitRoot, r.conf.Path)
	}

	if err := setter.Execute(
		path, types.NamespacedName{
			Namespace: hr.Namespace,
			Name:      hr.Name}, v,
	); err != nil {
		if err == setter.ErrMarkNotFound {
			return nil
		}
		return xerrors.Errorf("failed to set new version: %+w", err)
	}

	if s, _ := r.wt.Status(); s.String() == "" {
		return nil
	}

	if _, err := r.wt.Add("."); err != nil {
		return xerrors.Errorf("failed to add: %+w", err)
	}

	msg := fmt.Sprintf("Update HelmRelease %s/%s to %s", hr.Namespace, hr.Name, v)
	if r.conf.Prefix != "" {
		msg = fmt.Sprintf("[%s] %s", r.conf.Prefix, msg)
	}
	if _, err := r.wt.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  r.conf.Name,
			Email: r.conf.Email,
			When:  time.Now(),
		},
	}); err != nil {
		return xerrors.Errorf("failed to commit: %+w", err)
	}

	if err := r.repo.Push(&git.PushOptions{
		Auth:       &http.BasicAuth{Username: r.conf.GithubToken},
		Progress:   io.Discard,
		RemoteName: "origin",
	}); err != nil {
		return xerrors.Errorf("failed to push: %+w", err)
	}

	gh := github.NewClient(
		oauth2.NewClient(ctx, oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: r.conf.GithubToken},
		)),
	)

	pr := &github.NewPullRequest{
		Title: github.String(fmt.Sprintf("Update HelmRelease %s/%s", hr.Namespace, hr.Name)),
		Base:  github.String(r.head.Name().Short()),
		Head:  github.String(branchName),
	}

	owner, repo := parseRepoURL(r.conf.Target)
	if prs, _, _ := gh.PullRequests.List(ctx, owner, repo,
		&github.PullRequestListOptions{
			Base: r.head.Name().Short(),
			Head: branchName}); len(prs) > 0 {
		return nil
	}

	if _, _, err := gh.PullRequests.Create(ctx, owner, repo, pr); err != nil {
		return xerrors.Errorf("failed to PR: %+w", err)
	}

	return nil
}

// parseRepoURL decomposes the specific URL into owner and repository
func parseRepoURL(repoURL string) (string, string) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", ""
	}

	s := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(s) != 2 {
		return "", ""
	}

	return s[0], s[1]
}

func (r *helmVersionUpdater) checkout(branchName string) error {
	localRefName := plumbing.NewBranchReferenceName(branchName)
	remoteRefName := plumbing.NewRemoteReferenceName("origin", branchName)

	_, err := r.repo.Reference(remoteRefName, true)

	if err != nil {
		if e := r.wt.Checkout(&git.CheckoutOptions{Create: true, Hash: r.head.Hash(), Branch: localRefName}); e != nil {
			return xerrors.Errorf("failed to checkout: %+w", e)
		}
	} else {
		if e := r.repo.CreateBranch(&gitconfig.Branch{Name: branchName, Remote: "origin", Merge: localRefName}); e != nil {
			return xerrors.Errorf("failed to create branch: %+w", e)
		}
		sym := plumbing.NewSymbolicReference(localRefName, remoteRefName)
		if e := r.repo.Storer.SetReference(sym); e != nil {
			return xerrors.Errorf("failed to set reference: %+w", e)
		}

		if e := r.wt.Checkout(&git.CheckoutOptions{Branch: localRefName}); e != nil {
			return xerrors.Errorf("failed to checkout: %+w", e)
		}
	}

	return nil
}
