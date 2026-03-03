package main

import (
	"bytes"
	"fmt"
	"io"
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

	w := reporter.StreamWriter("terraform", "output")
	defer w.Close()

	start := time.Now()

	terraformDir := filepath.Join(root, "aws", "terraform")

	// Check if terraform directory exists.
	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		slog.Info("no terraform directory, skipping")
		fmt.Fprintf(w, "no terraform directory, skipping\n")
		reporter.FinishJob("terraform", FinishJobResult{
			Status:     "success",
			DurationMs: time.Since(start).Milliseconds(),
		})
		return nil
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
		reporter.FinishJob("terraform", FinishJobResult{
			Status:     "failed",
			DurationMs: time.Since(start).Milliseconds(),
			Error:      errMsg,
		})
		return fmt.Errorf("terraform init: %w", err)
	}

	// terraform apply -auto-approve
	// We need to capture the output for parsing resource counts while also
	// streaming it to the orchestrator.
	fmt.Fprintf(w, "=== terraform apply\n")
	var applyBuf bytes.Buffer
	applyTee := io.MultiWriter(w, &applyBuf)

	applyCmd := exec.Command("terraform", "apply", "-auto-approve")
	applyCmd.Dir = terraformDir
	applyCmd.Stdout = applyTee
	applyCmd.Stderr = applyTee
	err := applyCmd.Run()
	duration := time.Since(start).Milliseconds()

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = fmt.Sprintf("terraform apply: %v", err)
		fmt.Fprintf(w, "=== apply failed: %s\n", errMsg)
	}

	// Parse resource counts from output.
	added, changed, destroyed := parseTerraformOutput(applyBuf.String())

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
