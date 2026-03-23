FROM golang:1.26.1-alpine

RUN apk add --no-cache build-base gcc cmake git git-subtree bash nodejs npm \
    python3 py3-pip sqlite ca-certificates curl pkgconf tailscale protoc

# protoc-gen-go (for breadcrumbs protobuf codegen)
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11

# NLopt 2.10.0
RUN curl -L https://github.com/stevengj/nlopt/archive/refs/tags/v2.10.0.tar.gz | tar xz && \
    cd nlopt-2.10.0 && \
    cmake -DBUILD_SHARED_LIBS=ON -DNLOPT_PYTHON=OFF -DNLOPT_OCTAVE=OFF \
          -DNLOPT_MATLAB=OFF -DNLOPT_GUILE=OFF -DNLOPT_SWIG=OFF . && \
    make -j$(nproc) && make install && ldconfig /usr/local/lib && \
    cd / && rm -rf nlopt-2.10.0

# git-filter-repo (for publish)
RUN pip install --break-system-packages git-filter-repo

# jj (Jujutsu VCS)
RUN JJ_VERSION=$(curl -sL https://api.github.com/repos/jj-vcs/jj/releases/latest | grep '"tag_name"' | sed 's/.*"v\(.*\)".*/\1/') && \
    curl -LO "https://github.com/jj-vcs/jj/releases/download/v${JJ_VERSION}/jj-v${JJ_VERSION}-x86_64-unknown-linux-musl.tar.gz" && \
    tar xzf "jj-v${JJ_VERSION}-x86_64-unknown-linux-musl.tar.gz" --strip-components=0 -C /usr/local/bin/ ./jj && \
    rm "jj-v${JJ_VERSION}-x86_64-unknown-linux-musl.tar.gz"
RUN jj config set --user user.name "CI" && \
    jj config set --user user.email "ci@monks.co"

# terraform
RUN curl -LO https://releases.hashicorp.com/terraform/1.11.0/terraform_1.11.0_linux_amd64.zip && \
    unzip terraform_1.11.0_linux_amd64.zip -d /usr/local/bin/ && \
    rm terraform_1.11.0_linux_amd64.zip

# gh (GitHub CLI, for publish)
RUN GH_VERSION=$(curl -sL https://api.github.com/repos/cli/cli/releases/latest | grep '"tag_name"' | sed 's/.*"v\(.*\)".*/\1/') && \
    curl -LO "https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_amd64.tar.gz" && \
    tar xzf "gh_${GH_VERSION}_linux_amd64.tar.gz" --strip-components=2 -C /usr/local/bin/ "gh_${GH_VERSION}_linux_amd64/bin/gh" && \
    rm "gh_${GH_VERSION}_linux_amd64.tar.gz"

# claude (Claude Code CLI)
RUN npm install -g @anthropic-ai/claude-code

# codex (OpenAI Codex CLI)
RUN npm install -g @openai/codex

# flyctl
RUN curl -L https://fly.io/install.sh | sh
ENV FLYCTL_INSTALL="/root/.fly"
ENV PATH="$FLYCTL_INSTALL/bin:$PATH"

# Go modules (cached in Docker layer)
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build the builder binary
RUN go build -o /usr/local/bin/ci-builder ./apps/ci/cmd/builder

ENV MONKS_ROOT=/data/repo
ENV MONKS_DATA=/data
ENV GOMODCACHE=/data/gomodcache
ENV GOCACHE=/data/gobuildcache

# Incrementum config (so integration tests can reach the LLM gateway)
RUN mkdir -p /root/.config/incrementum
COPY <<'EOF' /root/.config/incrementum/config.toml
[llm]
model = "gpt-5.2-codex"

[[llm.providers]]
name = "tailnet-openai"
api = "openai-responses"
base-url = "https://ai.tail98579.ts.net"
models = ["gpt-5.2", "gpt-5.2-codex", "gpt-5-nano", "gpt-5", "gpt-5.1"]

[[llm.providers]]
name = "tailnet-anthropic"
api = "anthropic-messages"
base-url = "https://ai.tail98579.ts.net"
models = ["claude-sonnet-4-5-20250929", "claude-haiku-4-5-20251001", "claude-sonnet-4-5", "claude-haiku-4-5", "claude-opus-4-5"]
EOF

# Entrypoint: start kernel tailscale then exec the builder
COPY <<'SCRIPT' /usr/local/bin/entrypoint.sh
#!/bin/sh
set -e
tailscaled &
tailscale up --authkey="$TS_AUTHKEY" --hostname="monks-ci-builder-fly-${FLY_REGION:-ord}"
exec /usr/local/bin/ci-builder
SCRIPT
RUN chmod +x /usr/local/bin/entrypoint.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
