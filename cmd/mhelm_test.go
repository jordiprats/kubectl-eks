package cmd

import (
	"testing"

	"github.com/jordiprats/kubectl-eks/pkg/data"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
)

func TestHelmNamespace(t *testing.T) {
	assert.Equal(t, "default", helmNamespace("", false))
	assert.Equal(t, "kube-system", helmNamespace("kube-system", false))
	assert.Equal(t, "", helmNamespace("", true))
	assert.Equal(t, "", helmNamespace("kube-system", true))
}

func TestMatchesHelmVersions(t *testing.T) {
	helmRelease := &release.Release{Chart: &chart.Chart{Metadata: &chart.Metadata{
		Version:    "1.12.3",
		AppVersion: "2.4.1",
	}}}

	tests := []struct {
		name            string
		release         *release.Release
		chartVersion    string
		notChartVersion string
		appVersion      string
		notAppVersion   string
		want            bool
	}{
		{name: "no filters", release: helmRelease, want: true},
		{name: "chart substring", release: helmRelease, chartVersion: "1.12", want: true},
		{name: "chart mismatch", release: helmRelease, chartVersion: "1.13", want: false},
		{name: "excluded chart substring", release: helmRelease, notChartVersion: "1.12", want: false},
		{name: "app substring", release: helmRelease, appVersion: "2.4", want: true},
		{name: "excluded app substring", release: helmRelease, notAppVersion: "2.4", want: false},
		{name: "combined filters", release: helmRelease, chartVersion: "1.", appVersion: "2.", want: true},
		{name: "missing metadata with positive filter", release: &release.Release{}, chartVersion: "1", want: false},
		{name: "missing metadata with exclusion", release: &release.Release{}, notChartVersion: "1", want: true},
		{name: "nil release", chartVersion: "1", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := matchesHelmVersions(test.release, test.chartVersion, test.notChartVersion, test.appVersion, test.notAppVersion)
			assert.Equal(t, test.want, actual)
		})
	}
}

func TestLatestHelmReleases(t *testing.T) {
	releases := []*release.Release{
		{Name: "api", Namespace: "apps", Version: 1},
		{Name: "api", Namespace: "apps", Version: 3},
		{Name: "api", Namespace: "other", Version: 2},
		{Name: "web", Namespace: "apps", Version: 1},
		nil,
	}

	actual := latestHelmReleases(releases)

	assert.Len(t, actual, 3)
	assert.Equal(t, "api", actual[0].Name)
	assert.Equal(t, 3, actual[0].Version)
	assert.Equal(t, "web", actual[1].Name)
	assert.Equal(t, "other", actual[2].Namespace)
}

func TestHelmReleaseResult(t *testing.T) {
	cluster := data.ClusterInfo{AWSProfile: "prod", Region: "eu-west-1", ClusterName: "main"}
	helmRelease := &release.Release{
		Name:      "api",
		Namespace: "apps",
		Version:   4,
		Info:      &release.Info{Status: release.StatusDeployed},
		Chart: &chart.Chart{Metadata: &chart.Metadata{
			Name:       "api-chart",
			Version:    "1.2.3",
			AppVersion: "2.0.0",
		}},
	}

	actual := helmReleaseResult(cluster, helmRelease)

	assert.Equal(t, "prod", actual.Profile)
	assert.Equal(t, "deployed", actual.Status)
	assert.Equal(t, "api-chart-1.2.3", actual.Chart)
	assert.Equal(t, "2.0.0", actual.AppVersion)
	assert.Equal(t, "-", actual.Updated)
}

func TestHelmReleaseResultHandlesMissingMetadata(t *testing.T) {
	actual := helmReleaseResult(data.ClusterInfo{}, &release.Release{Name: "api"})

	assert.Equal(t, "unknown", actual.Status)
	assert.Equal(t, "-", actual.Chart)
	assert.Equal(t, "-", actual.AppVersion)
}
