// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"os"
)

const (
	HostEnvVar           = "CI_SPLUNK_HOST"
	UserEnvVar           = "CI_SPLUNK_USERNAME"
	PasswordEnvVar       = "CI_SPLUNK_PASSWORD" //nolint:gosec
	ManagementPortEnvVar = "CI_SPLUNK_PORT"
)

// GetConfigVariable returns the value of the environment variable with the given name.
func GetConfigVariable(variableName string) string {
	envVariableName := ""
	switch variableName {
	case "HOST":
		envVariableName = HostEnvVar
	case "USER":
		envVariableName = UserEnvVar
	case "PASSWORD":
		envVariableName = PasswordEnvVar
	case "MANAGEMENT_PORT":
		envVariableName = ManagementPortEnvVar
	}
	value := os.Getenv(envVariableName)

	if value != "" {
		return value
	}
	panic(envVariableName + " environment variable is not set")
}
