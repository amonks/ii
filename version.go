package main

import "fmt"

var buildChangeID = "unknown"
var buildCommitID = "unknown"
var buildDev string

func init() {
	rootCmd.Version = versionString()
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}

func isDev() bool {
	return buildDev == "true"
}

func versionString() string {
	if isDev() {
		return fmt.Sprintf("dev build\nchange_id %s\ncommit_id %s", buildChangeID, buildCommitID)
	}
	return fmt.Sprintf("change_id %s\ncommit_id %s", buildChangeID, buildCommitID)
}

// buildVersion returns the version string used in User-Agent headers.
// Dev builds use "dev" for a stable useragent; installed builds use "changeID:commitID".
func buildVersion() string {
	if isDev() {
		return "dev"
	}
	return buildChangeID + ":" + buildCommitID
}
