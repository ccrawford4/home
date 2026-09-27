# Use the trixie base for arm64
FROM --platform=linux/arm64 debian:trixie AS builder

ENV DEBIAN_FRONTEND=noninteractive

# Copy the Raspberry PI signing key (hailo-h10-all installation only works with this key)
COPY raspberrypi-archive-keyring.gpg /usr/share/keyrings/raspberrypi-archive-keyring.gpg
RUN echo "deb [signed-by=/usr/share/keyrings/raspberrypi-archive-keyring.gpg] http://archive.raspberrypi.com/debian trixie main" \
	> /etc/apt/sources.list.d/raspberrypi.list

# Base build dependencies
RUN apt update && \
	apt upgrade -y && \
	apt full-upgrade -y && \
	apt install -y \
	ca-certificates \
	cmake \
	g++ \
	git \
	libssl-dev \
	ninja-build \
	pkg-config \
	python3 \
	build-essential

# Install Hailo-10H runtime
RUN apt install -y dkms hailo-h10-all || true

ARG HAILO_OLLAMA_REPO=https://github.com/ccrawford4/hailo_model_zoo_genai.git
ARG HAILO_OLLAMA_REF=main

WORKDIR /src
RUN git clone "${HAILO_OLLAMA_REPO}" hailo_model_zoo_genai
WORKDIR /src/hailo_model_zoo_genai
RUN git checkout "${HAILO_OLLAMA_REF}"

# Build and package the server. The repo already contains the packaging/install logic.
RUN cmake -S . -B build -G Ninja -DHAILO_BUILD_UT=OFF && \
	cmake --build build --target hailo-ollama

FROM --platform=linux/arm64 debian:trixie

ENV DEBIAN_FRONTEND=noninteractive

COPY raspberrypi-archive-keyring.gpg /usr/share/keyrings/raspberrypi-archive-keyring.gpg
RUN echo "deb [signed-by=/usr/share/keyrings/raspberrypi-archive-keyring.gpg] http://archive.raspberrypi.com/debian trixie main" \
	> /etc/apt/sources.list.d/raspberrypi.list

RUN apt update && \
	apt upgrade -y && \
	apt full-upgrade -y && \
	apt install -y dkms hailo-h10-all || true

COPY --from=builder /src/hailo_model_zoo_genai/build/src/apps/server/hailo-ollama /usr/local/bin/hailo-ollama

# Keep the runtime command identical to the Helm chart.
CMD ["/bin/sh", "-c", "hailo-ollama"]
