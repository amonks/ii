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
func TerraformApply(root string, reporter *Reporter) error {
	reporter.StartJob("terraform", "terraform")
	start := time.Now()

	terraformDir := filepath.Join(root, "aws", "terraform")

	// Check if terraform directory exists.
	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		slog.Info("no terraform directory, skipping")
		reporter.FinishJob("terraform", FinishJobResult{
			Status:     "success",
			DurationMs: time.Since(start).Milliseconds(),
		})
		return nil
	}

	// terraform init
	initCmd := exec.Command("terraform", "init")
	initCmd.Dir = terraformDir
	if output, err := initCmd.CombinedOutput(); err != nil {
		reporter.FinishJob("terraform", FinishJobResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("terraform init: %v\n%s", err, string(output)),
		})
		return fmt.Errorf("terraform init: %w", err)
	}

	// terraform apply -auto-approve
	applyCmd := exec.Command("terraform", "apply", "-auto-approve")
	applyCmd.Dir = terraformDir
	output, err := applyCmd.CombinedOutput()
	duration := time.Since(start).Milliseconds()

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = fmt.Sprintf("terraform apply: %v", err)
	}

	// Parse resource counts from output.
	added, changed, destroyed := parseTerraformOutput(string(output))

	reporter.FinishJob("terraform", FinishJobResult{
		Status:     status,
		DurationMs: duration,
		Error:      errMsg,
		Terraform: &TerraformData{
			ResourcesAdded:     added,
			ResourcesChanged:   changed,
			ResourcesDestroyed: destroyed,
		},
	})

	if err != nil {
		return fmt.Errorf("terraform apply: %w", err)
	}
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
