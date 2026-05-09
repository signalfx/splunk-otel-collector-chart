// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package internal

import "testing"

func TestParseCollectorImage(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		image    string
		wantRepo string
		wantTag  string
	}{
		{
			name:     "tagged image",
			image:    "quay.io/signalfx/splunk-otel-collector-dev:latest",
			wantRepo: "quay.io/signalfx/splunk-otel-collector-dev",
			wantTag:  "latest",
		},
		{
			name:     "image without tag defaults to latest",
			image:    "quay.io/signalfx/splunk-otel-collector-dev",
			wantRepo: "quay.io/signalfx/splunk-otel-collector-dev",
			wantTag:  "latest",
		},
		{
			name:     "registry with port",
			image:    "localhost:5000/splunk-otel-collector-dev:abc123",
			wantRepo: "localhost:5000/splunk-otel-collector-dev",
			wantTag:  "abc123",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gotRepo, gotTag := parseCollectorImage(testCase.image)
			if gotRepo != testCase.wantRepo {
				t.Fatalf("repo mismatch: got %q want %q", gotRepo, testCase.wantRepo)
			}
			if gotTag != testCase.wantTag {
				t.Fatalf("tag mismatch: got %q want %q", gotTag, testCase.wantTag)
			}
		})
	}
}
