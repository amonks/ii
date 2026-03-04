package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

// TerraformApply runs terraform init and apply in the aws/terraform directory.
// It runs as a "terraform" stream within the "deploy" job.
func TerraformApply(root string, reporter *Reporter) error {
	reporter.StartStream("deploy", "terraform")

	w := reporter.StreamWriter("deploy", "terraform")
	defer w.Close()

	start := time.Now()

	terraformDir := filepath.Join(root, "aws", "terraform")

	// Check if terraform directory exists.
	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		slog.Info("no terraform directory, skipping")
		fmt.Fprintf(w, "no terraform directory, skipping\n")
		d := time.Since(start).Milliseconds()
		reporter.FinishStream("deploy", "terraform", FinishStreamResult{
			Status:     "success",
			DurationMs: d,
		})
		return nil
	}

	// Set TF_VAR_ env vars from available tokens.
	if v := os.Getenv("GANDI_TOKEN"); v != "" {
		os.Setenv("TF_VAR_GANDI_PERSONAL_ACCESS_TOKEN", v)
	}

	// terraform init
	fmt.Fprintf(w, "=== terraform init\n")
	initCmd := exec.Command("terraform", "init")
	initCmd.Dir = terraformDir
	initCmd.Stdout = w
	initCmd.Stderr = w
	if err := initCmd.Run(); err != nil {
		errMsg := fmt.Sprintf("terraform init: %v", err)
		fmt.Fprintf(w, "=== init failed: %s\n", errMsg)
		d := time.Since(start).Milliseconds()
		reporter.FinishStream("deploy", "terraform", FinishStreamResult{
			Status:     "failed",
			DurationMs: d,
			Error:      errMsg,
		})
		return fmt.Errorf("terraform init: %w", err)
	}

	// terraform apply -auto-approve
	fmt.Fprintf(w, "=== terraform apply\n")

	applyCmd := exec.Command("terraform", "apply", "-auto-approve")
	applyCmd.Dir = terraformDir
	applyCmd.Stdout = w
	applyCmd.Stderr = w
	err := applyCmd.Run()
	duration := time.Since(start).Milliseconds()

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = fmt.Sprintf("terraform apply: %v", err)
		fmt.Fprintf(w, "=== apply failed: %s\n", errMsg)
	}

	reporter.FinishStream("deploy", "terraform", FinishStreamResult{
		Status:     status,
		DurationMs: duration,
		Error:      errMsg,
	})

	if err != nil {
		return fmt.Errorf("terraform apply: %w", err)
	}
	fmt.Fprintf(w, "=== done (%dms)\n", duration)
	return nil
}

var terraformSummaryRe = regexp.MustCompile(`(\d+) added, (\d+) changed, (\d+) destroyed`)

func parseTerraformOutput(output string) (added, changed, destroyed int) {
	matches := terraformSummaryRe.FindStringSubmatch(output)
	if len(matches) == 4 {
		added, _ = strconv.Atoi(matches[1])
		changed, _ = strconv.Atoi(matches[2])
		destroyed, _ = strconv.Atoi(matches[3])
	}
	return
}
