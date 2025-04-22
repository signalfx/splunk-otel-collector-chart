// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var debug bool

const (
	EndOfLifeURL        string = "https://endoflife.date/api/kubernetes.json"
	KindDockerHubURL    string = "https://hub.docker.com/v2/repositories/kindest/node/tags?page_size=1&page=1&ordering=last_updated&name="
	MiniKubeURL         string = "https://raw.githubusercontent.com/kubernetes/minikube/master/pkg/minikube/constants/constants_kubernetes_versions.go"
	KubeKindVersion     string = "k8s-kind-version"
	KubeMinikubeVersion string = "k8s-minikube-version"
)

type KubernetesVersion struct {
	Cycle       string `json:"cycle"`
	ReleaseDate string `json:"releaseDate"`
	EOLDate     string `json:"eol"`
	Latest      string `json:"latest"`
}

type DockerImage struct {
	Count int `json:"count"`
}

// getSupportedKubernetesVersions returns the supported Kubernetes versions
// by checking the EOL date of the collected versions.
func getSupportedKubernetesVersions(url string) ([]KubernetesVersion, error) {
	body, err := getRequestBody(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s versions: %w", err)
	}
	var kubernetesVersions, supportedKubernetesVersions []KubernetesVersion
	if err = json.Unmarshal(body, &kubernetesVersions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	now := time.Now()
	for _, kubernetesVersion := range kubernetesVersions {
		eolDate, parseErr := time.Parse(time.DateOnly, kubernetesVersion.EOLDate)
		if parseErr != nil {
			return nil, fmt.Errorf("error parsing date: %w", parseErr)
		}
		if eolDate.After(now) {
			supportedKubernetesVersions = append(supportedKubernetesVersions, kubernetesVersion)
		} else {
			logDebug("Skipping version %s, EOL date %s", kubernetesVersion.Cycle, kubernetesVersion.EOLDate)
		}
	}
	return supportedKubernetesVersions, nil
}

// getLatestSupportedMinikubeVersions iterates through the K8s supported versions and find the latest minikube after parsing
// the sorted ValidKubernetesVersions slice from constants_kubernetes_versions.go
func getLatestSupportedMinikubeVersions(url string, k8sVersions []KubernetesVersion) ([]string, error) {
	body, err := getRequestBody(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get minikube versions: %w", err)
	}

	// Extract the slice using a regular expression
	re := regexp.MustCompile(`ValidKubernetesVersions = \[\]string{([^}]*)}`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return nil, errors.New("minikube, failed to find the Kubernetes versions slice")
	}

	// Parse and cleanup the slice values
	minikubeVersions := strings.Split(strings.NewReplacer("\n", "", `"`, "", "\t", "", " ", "").Replace(matches[1]), ",")

	logDebug("Found minikube versions: %s", minikubeVersions)

	var latestMinikubeVersions []string
	// the minikube version slice is sorted, break when first cycle match is found
	for _, k8sVersion := range k8sVersions {
		for _, minikubeVersion := range minikubeVersions {
			if strings.Contains(minikubeVersion, k8sVersion.Cycle) {
				latestMinikubeVersions = append(latestMinikubeVersions, minikubeVersion)
				break
			}
		}
	}

	return latestMinikubeVersions, nil
}

// getLatestSupportedKindImages iterates through the K8s supported versions and find the latest kind
// tag that supports that version
func getLatestSupportedKindImages(url string, k8sVersions []KubernetesVersion) ([]string, error) {
	var supportedKindVersions []string
	for _, k8sVersion := range k8sVersions {
		tag := k8sVersion.Latest
		for {
			exists, err := imageTagExists(url, tag)
			if err != nil {
				return supportedKindVersions, fmt.Errorf("failed to check image tag existence: %w", err)
			}
			if exists {
				supportedKindVersions = append(supportedKindVersions, "v"+tag)
				break
			}
			tag, err = decrementMinorMinorVersion(tag)
			if err != nil {
				// It's possible that kind still does not have a tag for new versions, break the loop and
				// process other k8s versions
				if strings.Contains(err.Error(), "minor version cannot be decremented below 0") {
					logDebug("No kind image found for k8s version %s", k8sVersion.Cycle)
					break
				}
				return supportedKindVersions, fmt.Errorf("failed to decrement k8sVersion: %w", err)
			}
		}
	}
	return supportedKindVersions, nil
}

func imageTagExists(url string, tag string) (bool, error) {
	body, err := getRequestBody(url + tag)
	if err != nil {
		return false, fmt.Errorf("failed to get image tag: %w", err)
	}

	var kindImage DockerImage
	if err = json.Unmarshal(body, &kindImage); err != nil {
		return false, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	if kindImage.Count > 0 {
		return true, nil
	}
	return false, nil
}

func decrementMinorMinorVersion(version string) (string, error) {
	parts := strings.Split(version, ".")
	if len(parts) < 3 {
		return "", fmt.Errorf("version does not have a minor version: %s", version)
	}

	minor, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid minor version: %s", parts[1])
	}

	if minor == 0 {
		return "", errors.New("minor version cannot be decremented below 0")
	}

	parts[2] = strconv.Itoa(minor - 1)
	return strings.Join(parts, "."), nil
}

func updateMatrixFile(filePath string, kindVersions []string, minikubeVersions []string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var testMatrix map[string]map[string][]string
	if err = json.Unmarshal(content, &testMatrix); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	for _, value := range testMatrix {
		if len(kindVersions) > 0 && value[KubeKindVersion] != nil {
			value[KubeKindVersion] = kindVersions
		} else if len(minikubeVersions) > 0 && value[KubeMinikubeVersion] != nil {
			value[KubeMinikubeVersion] = minikubeVersions
		}
	}
	// Marshal the updated test matrix back to JSON
	updatedContent, err := json.MarshalIndent(testMatrix, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated JSON: %w", err)
	}

	// Ensure the file ends with a new line to make the pre-commit check happy
	updatedContent = append(updatedContent, '\n')

	if err = os.WriteFile(filePath, updatedContent, 0o644); err != nil {
		return fmt.Errorf("failed to write updated file: %w", err)
	}
	return nil
}

func sortVersions(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		vi := strings.Split(versions[i][1:], ".") // Remove "v" and split by "."
		vj := strings.Split(versions[j][1:], ".")

		for k := 0; k < len(vi) && k < len(vj); k++ {
			if vi[k] != vj[k] {
				return vi[k] > vj[k] // Sort in descending order
			}
		}
		return len(vi) > len(vj)
	})
}

func getRequestBody(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	return body, nil
}

func logDebug(format string, v ...any) {
	if debug {
		log.Printf(format, v...)
	}
}

func main() {
	// setup logging
	flag.BoolVar(&debug, "debug", false, "Enable debug logging")
	flag.Parse()
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	k8sVersions, err := getSupportedKubernetesVersions(EndOfLifeURL)
	if err != nil || len(k8sVersions) == 0 {
		log.Fatalf("Failed to get k8s versions: %v", err)
	}
	logDebug("Found supported k8s versions %v", k8sVersions)

	kindVersions, err := getLatestSupportedKindImages(KindDockerHubURL, k8sVersions)
	if err != nil {
		log.Printf("failed to get all kind versions: %v", err)
	}
	if len(kindVersions) > 0 {
		// needs to be sorted so we don't end up with false positive diff in the json matrix file
		sortVersions(kindVersions)
		logDebug("Found supported kind images: %v", kindVersions)
	}

	minikubeVersions, err := getLatestSupportedMinikubeVersions(MiniKubeURL, k8sVersions)
	if err != nil {
		log.Printf("failed to get minikube versions: %v", err)
	}
	if len(minikubeVersions) > 0 {
		logDebug("Found supported minikube versions: %v", minikubeVersions)
	}

	if len(kindVersions) == 0 && len(minikubeVersions) == 0 {
		log.Fatalf("No supported versions found. Run with -debug=true for more info.")
	}

	path := "ci-matrix.json"
	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v ", err)
	}
	path = filepath.Join(currentDir, filepath.Clean(path))
	err = updateMatrixFile(path, kindVersions, minikubeVersions)
	if err != nil {
		log.Fatalf("Failed to update matrix file: %v", err)
	}
	os.Exit(0)
}
