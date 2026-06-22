package cli

import (
	"errors"
	"strings"
)

type codedError struct {
	code int
	msg  string
}

func (e codedError) Error() string {
	return e.msg
}

func usageError(msg string) error {
	return codedError{code: 2, msg: msg}
}

func dependencyError(msg string) error {
	return codedError{code: 3, msg: msg}
}

func clusterError(msg string) error {
	return codedError{code: 4, msg: msg}
}

func serviceError(msg string) error {
	return codedError{code: 5, msg: msg}
}

func readinessError(msg string) error {
	return codedError{code: 6, msg: msg}
}

func exitCode(err error) int {
	var coded codedError
	if errors.As(err, &coded) {
		return coded.code
	}
	return 1
}

func errorHint(err error) string {
	var coded codedError
	errors.As(err, &coded)
	switch exitCode(err) {
	case 2:
		if strings.HasPrefix(coded.msg, "--") || strings.Contains(coded.msg, "--format") {
			return ""
		}
		return "run pyahu init --preset platform, or pass --file when using a non-default stack path"
	case 3:
		return "run pyahu doctor to check local dependencies and busy ports"
	case 4:
		return "check k3d/Docker state, then rerun the command"
	case 5:
		return "run pyahu services or pyahu describe <service> to inspect the current service state"
	case 6:
		return "run pyahu status or pyahu describe <service> to see what is still becoming ready"
	default:
		return ""
	}
}
