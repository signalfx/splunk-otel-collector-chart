package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
