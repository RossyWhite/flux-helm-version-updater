package helm

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/hashicorp/go-version"
	"golang.org/x/xerrors"
	helmrepo "helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"
)

var (
	ErrAlreadyUpToDate = errors.New("current version is already up-to-date")
)

type chartRepo struct {
	url *url.URL
}

// NewChartRepository returns *chartRepo
func NewChartRepository(u string) (*chartRepo, error) {
	URL, err := url.Parse(u)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse URL: %+w", err)
	}

	return &chartRepo{url: URL}, nil
}

// FindLatestVersion fetch the latest chart version and compare with current version.
func (r *chartRepo) FindLatestVersion(chartname, currentver string) (string, error) {
	index, err := r.downloadIndex()
	if err != nil {
		return "", xerrors.Errorf("failed to get index: %+w", err)
	}

	latest, err := r.findLatestVersion(index, chartname)
	if err != nil {
		return "", xerrors.Errorf("failed to find latest version: %+w", err)
	}

	if !r.shouldUpdate(latest, currentver) {
		return "", ErrAlreadyUpToDate
	}

	return latest, nil
}

// downloadIndex downloads indexFile from specific repository
func (r *chartRepo) downloadIndex() (*helmrepo.IndexFile, error) {
	u := *r.url

	u.Path = filepath.Join(u.Path, "index.yaml")
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, xerrors.Errorf("http.Get failed: %+w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, xerrors.Errorf("failed to read body: %+w", err)
	}

	i := &helmrepo.IndexFile{}
	if err := yaml.UnmarshalStrict(b, i); err != nil {
		return nil, xerrors.Errorf("failed to unmarshal index: %+w", err)
	}

	i.SortEntries()

	return i, nil
}

// findLatestVersion get the latest chart version of specific chart
func (r *chartRepo) findLatestVersion(index *helmrepo.IndexFile, chartname string) (string, error) {
	versions, ok := index.Entries[chartname]
	if !ok || len(versions) == 0 {
		return "", helmrepo.ErrNoChartName
	}

	return versions[0].Version, nil
}

// shouldUpdate decides if it should be updated.
func (r *chartRepo) shouldUpdate(latest, current string) bool {
	latestVer, err := version.NewVersion(latest)
	if err != nil {
		return false
	}

	currentVer, err := version.NewVersion(current)
	if err != nil {
		return false
	}

	if currentVer.GreaterThanOrEqual(latestVer) {
		return false
	}

	return true
}
