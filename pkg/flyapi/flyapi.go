// Package flyapi is a thin REST client for the Fly Machines API.
package flyapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Client is a Fly Machines API client.
type Client struct {
	Token      string
	AppName    string
	HTTPClient *http.Client
	BaseURL    string
}

// NewClient creates a new Fly Machines API client with the default base URL.
func NewClient(token, appName string) *Client {
	return &Client{
		Token:      token,
		AppName:    appName,
		HTTPClient: http.DefaultClient,
		BaseURL:    "https://api.machines.dev/v1",
	}
}

// MachineCreateInput is the request body for creating a machine.
type MachineCreateInput struct {
	Name   string        `json:"name,omitempty"`
	Region string        `json:"region,omitempty"`
	Config MachineConfig `json:"config"`
}

// MachineConfig describes the configuration for a machine.
type MachineConfig struct {
	Image       string            `json:"image"`
	Cmd         []string          `json:"cmd,omitempty"`
	Guest       Guest             `json:"guest,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Mounts      []Mount           `json:"mounts,omitempty"`
	AutoDestroy bool              `json:"auto_destroy"`
	Restart     RestartPolicy     `json:"restart,omitempty"`
}

// Guest describes the VM resources for a machine.
type Guest struct {
	CPUKind  string `json:"cpu_kind,omitempty"`
	CPUs     int    `json:"cpus,omitempty"`
	MemoryMB int    `json:"memory_mb,omitempty"`
}

// Mount describes a volume mount.
type Mount struct {
	Volume string `json:"volume"`
	Path   string `json:"path"`
}

// RestartPolicy describes the restart behavior for a machine.
type RestartPolicy struct {
	Policy string `json:"policy,omitempty"`
}

// MachineInfo is the response body returned by the Fly Machines API for
// machine operations.
type MachineInfo struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	State     string         `json:"state"`
	Region    string         `json:"region"`
	CreatedAt string         `json:"created_at"`
	Config    MachineConfig  `json:"config"`
	Events    []MachineEvent `json:"events,omitempty"`
}

// MachineEvent is a lifecycle event for a machine.
type MachineEvent struct {
	Type      string `json:"type"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

// APIError is returned when the Fly Machines API responds with a non-2xx
// status code.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("fly api: %d %s", e.StatusCode, e.Message)
}

// CreateMachine creates a new machine in the app.
// POST /apps/{app}/machines
func (c *Client) CreateMachine(ctx context.Context, input MachineCreateInput) (*MachineInfo, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := fmt.Sprintf("%s/apps/%s/machines", c.BaseURL, c.AppName)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, readAPIError(resp)
	}

	var info MachineInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &info, nil
}

// WaitForState blocks until the machine reaches the target state or the
// timeout elapses.
// GET /apps/{app}/machines/{id}/wait?state={state}&timeout={seconds}
func (c *Client) WaitForState(ctx context.Context, machineID, targetState string, timeout time.Duration) error {
	seconds := int(timeout.Seconds())
	url := fmt.Sprintf("%s/apps/%s/machines/%s/wait?state=%s&timeout=%s",
		c.BaseURL, c.AppName, machineID, targetState, strconv.Itoa(seconds))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readAPIError(resp)
	}
	return nil
}

// GetMachine retrieves information about a machine.
// GET /apps/{app}/machines/{id}
func (c *Client) GetMachine(ctx context.Context, machineID string) (*MachineInfo, error) {
	url := fmt.Sprintf("%s/apps/%s/machines/%s", c.BaseURL, c.AppName, machineID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, readAPIError(resp)
	}

	var info MachineInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &info, nil
}

// StopMachine stops a running machine.
// POST /apps/{app}/machines/{id}/stop
func (c *Client) StopMachine(ctx context.Context, machineID string) error {
	url := fmt.Sprintf("%s/apps/%s/machines/%s/stop", c.BaseURL, c.AppName, machineID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readAPIError(resp)
	}
	return nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

func readAPIError(resp *http.Response) *APIError {
	body, _ := io.ReadAll(resp.Body)
	msg := string(body)
	if msg == "" {
		msg = http.StatusText(resp.StatusCode)
	}
	return &APIError{
		StatusCode: resp.StatusCode,
		Message:    msg,
	}
}
