package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_UpdateMatrixFile_PreservesIPv6Coverage(t *testing.T) {
	original, err := os.ReadFile(filepath.Join("..", "..", ciMatrixPath))
	require.NoError(t, err)
	var originalMatrix map[string]map[string]any
	require.NoError(t, json.Unmarshal(original, &originalMatrix))
	var originalIncludes []map[string]any
	functionalMatrix := originalMatrix["functional_test_v2"]
	includedJSON, err := json.Marshal(functionalMatrix["include"])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(includedJSON, &originalIncludes))
	require.Len(t, originalIncludes, 1)
	require.Equal(t, "histogram", originalIncludes[0]["test-job"])
	require.Equal(t, "ipv6", originalIncludes[0]["ip-family"])

	for _, tc := range []struct {
		name         string
		kindVersions []string
	}{
		{
			name:         "kind version update",
			kindVersions: []string{"v1.37.0", "v1.36.2", "v1.35.6"},
		},
		{
			name: "minikube only update",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ciMatrixPath)
			require.NoError(t, os.WriteFile(path, original, 0o600))
			minikubeVersions := []string{"v1.37.1", "v1.36.3", "v1.35.7"}
			require.NoError(t, updateMatrixFile(path, tc.kindVersions, minikubeVersions))

			updated, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			var updatedMatrix map[string]map[string]any
			require.NoError(t, json.Unmarshal(updated, &updatedMatrix))
			matrix := updatedMatrix["functional_test_v2"]
			assert.Equal(t, functionalMatrix["test-job"], matrix["test-job"])
			assert.Equal(t, functionalMatrix["exclude"], matrix["exclude"])
			assert.Equal(t, originalMatrix["kubeconform_tests"], updatedMatrix["kubeconform_tests"])
			assert.Equal(t, originalMatrix["functional_test"]["container_runtime"], updatedMatrix["functional_test"]["container_runtime"])
			assert.Equal(t, originalMatrix["functional_test"]["splunk_version"], updatedMatrix["functional_test"]["splunk_version"])
			assert.Equal(t, byte('\n'), updated[len(updated)-1])

			includes, validIncludes := matrix["include"].([]any)
			require.True(t, validIncludes)
			require.Len(t, includes, 1)
			include, validInclude := includes[0].(map[string]any)
			require.True(t, validInclude)
			assert.Equal(t, "histogram", include["test-job"])
			assert.Equal(t, "ipv6", include["ip-family"])
			if len(tc.kindVersions) == 0 {
				assert.Equal(t, functionalMatrix[kubeKindVersion], matrix[kubeKindVersion])
				assert.Equal(t, originalIncludes[0][kubeKindVersion], include[kubeKindVersion])
			} else {
				assert.Equal(t, tc.kindVersions[0], include[kubeKindVersion])
				versions, validVersions := matrix[kubeKindVersion].([]any)
				require.True(t, validVersions)
				require.Len(t, versions, len(tc.kindVersions))
				for i, version := range tc.kindVersions {
					assert.Equal(t, version, versions[i])
				}
				assert.Equal(t, versions, updatedMatrix["migration_tests"][kubeKindVersion])
			}
			minikube, validMinikube := updatedMatrix["functional_test"][kubeMinikubeVersion].([]any)
			require.True(t, validMinikube)
			require.Len(t, minikube, len(minikubeVersions))
			for i, version := range minikubeVersions {
				assert.Equal(t, version, minikube[i])
			}

			require.NoError(t, updateMatrixFile(path, tc.kindVersions, minikubeVersions))
			repeated, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, updated, repeated, "repeated updates should not create another PR diff")
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
