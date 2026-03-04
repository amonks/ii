package job

import internalstrings "github.com/amonks/incrementum/internal/strings"

func normalizeStage(stage Stage) Stage {
	return Stage(internalstrings.NormalizeLower(string(stage)))
}

func normalizeStatus(status Status) Status {
	return Status(internalstrings.NormalizeLower(string(status)))
}
