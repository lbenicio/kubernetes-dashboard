# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This is a Dockerfile for running and building Kubernetes dashboard
# It installs all deps in the container and adds the dashboard source
# This way it abstracts all required build tools away

# Keep the standalone development container on the same Go release as the modules.
FROM golang:1.27.3-bookworm

# Install Node.js. Go is already installed.
# A small tweak, apt-get update is already run by the nodejs setup script,
# so there's no need to run it again.
RUN apt-get update && apt-get install -y ca-certificates --no-install-recommends curl gnupg
RUN mkdir -p /etc/apt/keyrings
RUN curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key | \
  gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg
ENV NODE_MAJOR=24
RUN echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_$NODE_MAJOR.x nodistro main" | \
  tee /etc/apt/sources.list.d/nodesource.list
RUN apt-get update && apt-get install -y --no-install-recommends \
	nodejs \
	patch \
	chromium \
	bc \
	sudo \
	git \
	gosu \
	nano \
	less \
	xvfb \
	libgtk-3-0 \
	libgconf-2-4 \
	bzip2 \
	xz-utils \
	&& rm -rf /var/lib/apt/lists/* \
	&& apt-get clean

# Install yarn
RUN npm install -g yarn

ENV GIT_EDITOR=nano

# Install Firefox from Mozilla binaries and discard the archive in the same layer.
RUN mkdir -p /usr/local/lib/firefox && \
  wget "https://download.mozilla.org/?product=firefox-latest-ssl&os=linux64&lang=en-US" -O /tmp/firefox.tar.xz && \
  tar -xJf /tmp/firefox.tar.xz -C /usr/local/lib && \
  rm /tmp/firefox.tar.xz && \
  ln -s /usr/local/lib/firefox/firefox /usr/local/bin/firefox

# Set environment variable for JavaScript tests.
ENV CHROME_BIN=/usr/bin/chromium

# Set environment variable for terminal
ENV TERM=xterm

# Add ${GOPATH}/bin into ${PATH}
ENV PATH=${GOPATH}/bin:${PATH}

# Suppress angular analytics dialog
ENV NG_CLI_ANALYTICS=false

# Download architecture-matched Docker CLI, Compose, and kubectl binaries.
# Keep kubectl aligned with the Kubernetes version used by the local Kind cluster.
ARG DOCKER_CLI_VERSION=29.9.2
ARG DOCKER_COMPOSE_VERSION=v5.7.0
ARG KUBECTL_VERSION=v1.37.0
RUN set -eu; \
  case "$(dpkg --print-architecture)" in \
    amd64) DOCKER_ARCH=x86_64; COMPOSE_ARCH=x86_64; KUBECTL_ARCH=amd64 ;; \
    arm64) DOCKER_ARCH=aarch64; COMPOSE_ARCH=aarch64; KUBECTL_ARCH=arm64 ;; \
    *) echo "Unsupported development image architecture: $(dpkg --print-architecture)" >&2; exit 1 ;; \
  esac; \
  mkdir -p /usr/local/lib/docker/cli-plugins; \
  curl -fsSL "https://download.docker.com/linux/static/stable/${DOCKER_ARCH}/docker-${DOCKER_CLI_VERSION}.tgz" -o /tmp/docker.tgz; \
  tar -xzf /tmp/docker.tgz -C /tmp docker/docker; \
  install -m 0755 /tmp/docker/docker /usr/bin/docker; \
  rm -rf /tmp/docker /tmp/docker.tgz; \
  COMPOSE_URL="https://github.com/docker/compose/releases/download/${DOCKER_COMPOSE_VERSION}/docker-compose-linux-${COMPOSE_ARCH}"; \
  curl -fsSL "${COMPOSE_URL}" -o /tmp/docker-compose; \
  curl -fsSL "${COMPOSE_URL}.sha256" -o /tmp/docker-compose.sha256; \
  printf '%s  %s\n' "$(awk '{print $1}' /tmp/docker-compose.sha256)" /tmp/docker-compose | sha256sum -c -; \
  install -m 0755 /tmp/docker-compose /usr/local/lib/docker/cli-plugins/docker-compose; \
  rm /tmp/docker-compose /tmp/docker-compose.sha256; \
  KUBECTL_URL="https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${KUBECTL_ARCH}/kubectl"; \
  curl -fsSL "${KUBECTL_URL}" -o /tmp/kubectl; \
  curl -fsSL "${KUBECTL_URL}.sha256" -o /tmp/kubectl.sha256; \
  printf '%s  %s\n' "$(cat /tmp/kubectl.sha256)" /tmp/kubectl | sha256sum -c -; \
  install -m 0755 /tmp/kubectl /usr/local/bin/kubectl; \
  rm /tmp/kubectl /tmp/kubectl.sha256

# Install golangci for checking or fixing Go code.
# The version matches modules/common/tools/go.mod and is built with this Go toolchain.
# This install is also needed when running checks directly inside the container.
RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

# Enable go mod.
ENV GO111MODULE=on

# Install delve for debuging go files.
RUN go install github.com/go-delve/delve/cmd/dlv@v1.27.2

# Set GOPROXY to ensure download modules
ENV GOPROXY=https://proxy.golang.org

# Set NODE_OPTIONS to increase NodeJS heap size
ENV NODE_OPTIONS=--max-old-space-size=8192

# To install go modules by user, add write access to $GOPATH (default: 755)
# `chmod +w` does not work, so set 777.
RUN chmod 777 -R /go

# Volume for source code
VOLUME ["/go/src/github.com/kubernetes/dashboard"]

# Mount point for kubeconfig
RUN mkdir -p /home/user/.kube

# Current directory is always dashboard source directory.
WORKDIR /go/src/github.com/kubernetes/dashboard

# Run gosu command in container.
CMD ["./hack/develop/gosu-command.sh"]
