package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_UpdateMatrixFile_PreservesIPv6Coverage(t *testing.T) {
	const original = `{"functional_test_v2": {
		"k8s-kind-version": ["v1.36.1", "v1.35.5"],
		"include": [{"test-job": "histogram", "k8s-kind-version": "v1.36.1", "ip-family": "ipv6"}]
	}}`
	for _, tc := range []struct {
		name         string
		kindVersions []string
		want         string
	}{
		{
			name:         "kind version update",
			kindVersions: []string{"v1.37.0", "v1.36.2"},
			want: `{"functional_test_v2": {
				"k8s-kind-version": ["v1.37.0", "v1.36.2"],
				"include": [{"test-job": "histogram", "k8s-kind-version": "v1.37.0", "ip-family": "ipv6"}]
			}}`,
		},
		{
			name: "minikube only update",
			want: original,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ciMatrixPath)
			require.NoError(t, os.WriteFile(path, []byte(original), 0o600))
			require.NoError(t, updateMatrixFile(path, tc.kindVersions, []string{"v1.37.1"}))
			updated, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(updated))
		})
	}
}

type mockResponse struct {
	responseBody       []byte
	responseStatusCode int
	responseError      string
	responseErrorCode  int
}

func (mResponse mockResponse) setupMockServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(mResponse.responseStatusCode)
		if _, err := w.Write(mResponse.responseBody); err != nil {
			http.Error(w, mResponse.responseError, mResponse.responseErrorCode)
		}
	}))
}

func Test_FetchKubernetesVersions_ValidURL_ReturnsSupportedVersions(t *testing.T) {
	mResponse := mockResponse{
		responseBody: []byte(`	[{"cycle":"1.24","releaseDate":"2022-05-03","eol":"2023-10-03","latest":"1.24.8"},
								{"cycle":"1.34","releaseDate":"2025-05-03","eol":"2034-10-03","latest":"1.34.9"}]`),
		responseError:      "failed to write response",
		responseStatusCode: http.StatusOK,
		responseErrorCode:  http.StatusInternalServerError,
	}
	mockServer := mResponse.setupMockServer()
	defer mockServer.Close()

	versions, err := getSupportedKubernetesVersions(mockServer.URL)
	require.NoError(t, err)
	require.Len(t, versions, 1)
	assert.Equal(t, "1.34", versions[0].Cycle)
}

func Test_FetchKubernetesVersions_InvalidURL_ReturnsError(t *testing.T) {
	_, err := getSupportedKubernetesVersions("http:/12.168.1.2:2025/invalid")
	assert.Error(t, err)
}

func Test_GetSupportedKubernetesVersions_KeepsNewestThreeCycles(t *testing.T) {
	futureEOL := time.Now().AddDate(1, 0, 0).Format(time.DateOnly)
	pastEOL := time.Now().AddDate(-1, 0, 0).Format(time.DateOnly)
	for _, tc := range []struct {
		name     string
		versions []KubernetesVersion
		want     []string
	}{
		{
			name: "overlapping EOL periods in unsorted response",
			versions: []KubernetesVersion{
				{Cycle: "1.34", ReleaseDate: "2025-08-27", EOLDate: futureEOL},
				{Cycle: "1.36", ReleaseDate: "2026-04-22", EOLDate: futureEOL},
				{Cycle: "1.37", ReleaseDate: "2026-08-26", EOLDate: futureEOL},
				{Cycle: "1.35", ReleaseDate: "2025-12-17", EOLDate: futureEOL},
			},
			want: []string{"1.37", "1.36", "1.35"},
		},
		{
			name: "fewer than three supported cycles",
			versions: []KubernetesVersion{
				{Cycle: "1.35", ReleaseDate: "2025-12-17", EOLDate: futureEOL},
				{Cycle: "1.36", ReleaseDate: "2026-04-22", EOLDate: futureEOL},
				{Cycle: "1.34", ReleaseDate: "2025-08-27", EOLDate: pastEOL},
			},
			want: []string{"1.36", "1.35"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.versions)
			require.NoError(t, err)
			mockServer := (mockResponse{
				responseBody:       body,
				responseStatusCode: http.StatusOK,
			}).setupMockServer()
			defer mockServer.Close()

			versions, err := getSupportedKubernetesVersions(mockServer.URL)
			require.NoError(t, err)
			var cycles []string
			for _, version := range versions {
				cycles = append(cycles, version.Cycle)
			}
			assert.Equal(t, tc.want, cycles)
		})
	}
}

func Test_GetSupportedKubernetesVersions_EmptyResponse_ReturnsNoVersions(t *testing.T) {
	mResponse := mockResponse{
		responseBody:       []byte(`[]`),
		responseStatusCode: http.StatusOK,
		responseError:      "failed to write response",
		responseErrorCode:  http.StatusInternalServerError,
	}
	mockServer := mResponse.setupMockServer()
	defer mockServer.Close()

	versions, err := getSupportedKubernetesVersions(mockServer.URL)
	require.NoError(t, err)
	assert.Empty(t, versions)
}

func Test_GetSupportedKubernetesVersions_InvalidEOL_ReturnsError(t *testing.T) {
	mResponse := mockResponse{
		responseBody:       []byte(`[{"cycle":"1.24","releaseDate":"2022-05-03","eol":"202001-01","latest":"1.24.0"}]`),
		responseStatusCode: http.StatusOK,
		responseError:      "failed to write response",
		responseErrorCode:  http.StatusInternalServerError,
	}
	mockServer := mResponse.setupMockServer()
	defer mockServer.Close()

	versions, err := getSupportedKubernetesVersions(mockServer.URL)
	require.Error(t, err)
	assert.Nil(t, versions)
}

func Test_GetLatestSupportedMinikubeVersions_ValidResponse_ReturnsMatchingVersions(t *testing.T) {
	mResponse := mockResponse{
		responseBody:       []byte(`ValidKubernetesVersions = []string{"v1.24.3-alpha1", "v1.24.2", "v1.23.5", "v1.22.1"}`),
		responseStatusCode: http.StatusOK,
		responseError:      "failed to write response",
		responseErrorCode:  http.StatusInternalServerError,
	}
	mockServer := mResponse.setupMockServer()
	defer mockServer.Close()

	k8sVersions := []KubernetesVersion{
		{Cycle: "1.24"},
		{Cycle: "1.23"},
		{Cycle: "1.25"},
	}

	versions, err := getLatestSupportedMinikubeVersions(mockServer.URL, k8sVersions)
	require.NoError(t, err)
	assert.Equal(t, []string{"v1.24.3-alpha1", "v1.23.5"}, versions)
}

func Test_GetLatestSupportedMinikubeVersions_InvalidResponseFormat_ReturnsError(t *testing.T) {
	mResponse := mockResponse{
		responseBody:       []byte(`InvalidFormat`),
		responseStatusCode: http.StatusOK,
		responseError:      "failed to write response",
		responseErrorCode:  http.StatusInternalServerError,
	}
	mockServer := mResponse.setupMockServer()
	defer mockServer.Close()

	k8sVersions := []KubernetesVersion{
		{Cycle: "1.24"},
	}

	versions, err := getLatestSupportedMinikubeVersions(mockServer.URL, k8sVersions)
	assert.Error(t, err)
	assert.Nil(t, versions)
}

func Test_GetLatestSupportedKindImages_ValidResponse_ReturnsMatchingImages(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/image&name=1.24.2" {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(`{"count": 1}`)); err != nil {
				http.Error(w, "failed to write response", http.StatusInternalServerError)
			}
		} else {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(`{"count": 0}`)); err != nil {
				http.Error(w, "failed to write response", http.StatusInternalServerError)
			}
		}
	}))
	defer mockServer.Close()

	k8sVersions := []KubernetesVersion{
		{Cycle: "1.24", Latest: "1.24.3"},
		{Cycle: "1.23", Latest: "1.23.5"},
	}

	images, err := getLatestSupportedKindImages(mockServer.URL+"/image&name=", k8sVersions)
	require.NoError(t, err)
	assert.Equal(t, []string{"v1.24.2"}, images)
}
