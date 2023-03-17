package konfig

import (
	"cloud.google.com/go/compute/metadata"
	"os"
)

func RunningOnCloud() bool {
	if RunningOnAWS() || metadata.OnGCE() {
		return true
	}
	return false
}

func RunningOnAWS() bool {
	_, isSet := os.LookupEnv("AWS_EXECUTION_ENV")
	return isSet
}
