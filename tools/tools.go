// Copyright  Splunk, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build tools
// +build tools

package tools

// based on https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/internal/tools/tools.go
import (
	_ "github.com/client9/misspell/cmd/misspell"
	_ "github.com/daixiang0/gci"
	_ "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
	_ "golang.org/x/tools/cmd/goimports"
	_ "golang.org/x/vuln/cmd/govulncheck"
	_ "mvdan.cc/gofumpt"
)
