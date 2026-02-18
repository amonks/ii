# Internal Config

## Overview
The config package loads project and global configuration files and runs hook scripts.

## Configuration Model
- `Config` holds workspace, job, agent, and LLM configuration.
- `Workspace` defines `on-create` and `on-acquire` scripts.
- `Job` defines `test-commands`, the optional default `model`, and optional per-stage
  models (`implementation-model`, `code-review-model`, `project-review-model`).
- `Agent` defines agent defaults like the model and prompt cache retention (`cache-retention`).
- `LLM` defines LLM providers available for use.

### LLM Configuration

```toml
[[llm.providers]]
name = "anthropic"
api = "anthropic-messages"
base-url = "https://api.anthropic.com"
api-key-command = "op read op://Private/Anthropic/credential"
models = ["claude-sonnet-4-20250514", "claude-haiku-4-20250514"]
```

Each provider has:
- `name`: Unique identifier for the provider configuration
- `api`: API style (`anthropic-messages`, `openai-completions`, `openai-responses`)
- `base-url`: API endpoint
- `api-key-command`: Command to run to get API key (optional; if empty, no auth is used)
- `models`: List of model IDs available through this provider

## Agent Configuration

```toml
[agent]
model = "claude-haiku-4-5"
cache-retention = "short"
```

- `model` selects the default agent model when no task-specific model is set.
- `cache-retention` controls prompt caching for agent runs ("none", "short", "long"; default "short").

## Behavior

- `Load` reads either `incrementum.toml` or `.incrementum/config.toml` from the repo root and `~/.config/incrementum/config.toml`, then merges them.
- `LoadGlobal` reads only the global config file (useful when no repo context is available).
- If both `incrementum.toml` and `.incrementum/config.toml` exist, `Load` returns an error.
- Project values override global values, including explicitly empty strings or lists; missing configs return an empty config.
- Agent `cache-retention` values merge the same way as other agent fields (project overrides global, even when empty), and default to "short" at runtime if left unset.
- LLM providers are merged: project providers with the same name override global providers; providers are returned with project providers first, then remaining global providers.
- TOML decoding errors are surfaced with context.
- `RunScript` executes hook scripts in a target directory.
- Scripts honor a shebang line; otherwise `/bin/bash` is used.
- Script content is passed via stdin, with stdout/stderr forwarded to the caller.
- Job workflows require `job.test-commands` to be present and non-empty.
