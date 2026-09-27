# Use the trixie base for arm64
FROM --platform=linux/arm64 debian:trixie AS base

ENV DEBIAN_FRONTEND=noninteractive

# Copy the Raspberry PI signing key (hailo-h10-all installation only works with this key)
COPY raspberrypi-archive-keyring.gpg /usr/share/keyrings/raspberrypi-archive-keyring.gpg
RUN echo "deb [signed-by=/usr/share/keyrings/raspberrypi-archive-keyring.gpg] http://archive.raspberrypi.com/debian trixie main" \
	> /etc/apt/sources.list.d/raspberrypi.list

# Common runtime dependencies shared by both stages.
RUN apt-get update && \
	apt-get install -y --no-install-recommends \
	ca-certificates

FROM base AS builder

# Build dependencies
RUN apt-get update && \
	apt-get install -y --no-install-recommends \
	build-essential \
	cmake \
	g++ \
	git \
	libssl-dev \
	ninja-build \
	pkg-config \
	python3

# Hailo SDK/runtime needed at build time for CMake's HailoRT discovery.
RUN apt-get update && \
	apt-get install -y --no-install-recommends dkms hailo-h10-all || true

ARG HAILO_OLLAMA_REPO=https://github.com/ccrawford4/hailo_model_zoo_genai.git
ARG HAILO_OLLAMA_REF=main

WORKDIR /src
RUN git clone "${HAILO_OLLAMA_REPO}" hailo_model_zoo_genai
WORKDIR /src/hailo_model_zoo_genai
RUN git checkout "${HAILO_OLLAMA_REF}"

# Build and package the server. The repo already contains the packaging/install logic.
RUN cmake -S . -B build -G Ninja -DHAILO_BUILD_UT=OFF && \
	cmake --build build --target hailo-ollama

FROM base

# Install the Hailo runtime only in the final image.
RUN apt-get update && \
	apt-get install -y --no-install-recommends dkms hailo-h10-all || true

# Set the working directory for the runtime container
WORKDIR /app

# Copy artifacts
COPY --from=builder /src/hailo_model_zoo_genai/build/src/apps/server/hailo-ollama /usr/local/bin/hailo-ollama
COPY hailo_gen_ai_model_zoo_5.1.1_arm64.deb /app/

# Setup the GenAI model package and install the deps
RUN dpkg -i hailo_gen_ai_model_zoo_5.1.1_arm64.deb || \
	apt --fix-broken install -y

# Override packaged manifests with repo versions
COPY --from=builder /src/hailo_model_zoo_genai/models/manifests /usr/share/hailo-ollama/models/manifests

# Keep the runtime command identical to the Helm chart.
CMD ["/bin/sh", "-c", "hailo-ollama"]
