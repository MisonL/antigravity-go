package rpc

import "strings"

var deprecatedNoiseMethods = []string{
	"getusermemories",
	"getallcascadetrajectories",
}

func ShouldSilenceDeprecatedMethodError(method string, err error) bool {
	if err == nil {
		return false
	}

	return shouldSilenceDeprecatedNoise(method, err.Error())
}

func ShouldSilenceDeprecatedLogLine(line string) bool {
	return shouldSilenceDeprecatedNoise("", line)
}

func shouldSilenceDeprecatedNoise(method string, message string) bool {
	if message == "" {
		return false
	}

	normalized := strings.ToLower(strings.TrimSpace(message))
	if !strings.Contains(normalized, "deprecated") {
		return false
	}

	if method != "" {
		return isDeprecatedNoiseMethod(method)
	}

	for _, candidate := range deprecatedNoiseMethods {
		if strings.Contains(normalized, candidate) {
			return true
		}
	}

	return false
}

func isDeprecatedNoiseMethod(method string) bool {
	normalized := strings.ToLower(strings.TrimSpace(method))
	for _, candidate := range deprecatedNoiseMethods {
		if normalized == candidate {
			return true
		}
	}
	return false
}
