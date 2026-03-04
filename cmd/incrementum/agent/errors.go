package agent

import "errors"

// ErrNoModelConfigured indicates no model could be resolved from any source.
var ErrNoModelConfigured = errors.New("no model configured")
