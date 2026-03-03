FROM golang:1.26-alpine

RUN apk add --no-cache build-base gcc cmake git bash nodejs npm \
    python3 py3-pip sqlite ca-certificates curl

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

ENV MONKS_ROOT=/app
ENV MONKS_DATA=/data
CMD ["/usr/local/bin/ci-builder"]
