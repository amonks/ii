package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func Prompt(prompt string) (string) {
	var result string
	fmt.Printf("%s > ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		result = scanner.Text()
	}
	return result
}

func Select(prompt string, options []string) (string, error) {
	var result strings.Builder

	cmd := exec.Command("fzf", fmt.Sprintf("--prompt=%s >", prompt))
	cmd.Stdout = &result
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	input := strings.Join(options, "\n")
	_, err = io.Copy(stdin, strings.NewReader(input))

	if err != nil {
		return "", err
	}

	err = stdin.Close()
	if err != nil {
		return "", err
	}

	err = cmd.Start()
	if err != nil {
		return "", err
	}

	err = cmd.Wait()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(result.String()), nil
}
