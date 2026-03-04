package job

import internalstrings "monks.co/incrementum/internal/strings"

func normalizeStage(stage Stage) Stage {
	return Stage(internalstrings.NormalizeLower(string(stage)))
}

func normalizeStatus(status Status) Status {
	return Status(internalstrings.NormalizeLower(string(status)))
}
