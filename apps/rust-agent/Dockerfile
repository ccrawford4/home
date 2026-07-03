# Use Rust Official Image
FROM rust:1.92 AS chef

# Install cargo-chef for depency caching
RUN cargo install cargo-chef --locked
WORKDIR /usr/src/app

# Create a planner stage
FROM chef AS planner
COPY . .
RUN cargo chef prepare --recipe-path recipe.json

# Build and cache dependencies
FROM chef AS builder
ARG TARGETARCH
RUN case "$TARGETARCH" in \
      amd64) echo x86_64-unknown-linux-gnu ;; \
      arm64) echo aarch64-unknown-linux-gnu ;; \
      *) echo "Unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; \
    esac > /rust_target && \
    rustup target add $(cat /rust_target)

COPY --from=planner /usr/src/app/recipe.json recipe.json
RUN cargo chef cook --release --target $(cat /rust_target) --recipe-path recipe.json -j 2

# Build the source code
COPY . .
RUN cargo build --release --target $(cat /rust_target) -j 2 && \
    cp target/$(cat /rust_target)/release/rust-agent /usr/local/bin/rust-agent

# Use a slimmer image and install runtime dependencies
FROM rust:1.92-slim
RUN apt-get update && apt-get install -y \
    libssl3 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user with home directory
RUN useradd -m -s /bin/bash rustagent

# Copy and set permissions for rust-agent binary
COPY --from=builder /usr/local/bin/rust-agent /usr/local/bin/rust-agent
RUN chmod +x /usr/local/bin/rust-agent && \
    chown rustagent:rustagent /usr/local/bin/rust-agent

USER rustagent
CMD ["rust-agent"]
