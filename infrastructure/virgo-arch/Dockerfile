# Use the trixie base for arm64
FROM --platform=linux/arm64 debian:trixie

# Ensure that there is no need to pass in -y explicitly
ENV DEBIAN_FRONTEND=noninteractive

# Copy the Rasberry PI signing key (hailo-h10-all installation only works with this key)
COPY raspberrypi-archive-keyring.gpg /usr/share/keyrings/raspberrypi-archive-keyring.gpg
RUN echo "deb [signed-by=/usr/share/keyrings/raspberrypi-archive-keyring.gpg] http://archive.raspberrypi.com/debian trixie main" \
	> /etc/apt/sources.list.d/raspberrypi.list

# Upgrade deps
RUN apt update && \
	apt upgrade -y && \
	apt full-upgrade -y

# Install Hailo-10H runtime
RUN apt install -y dkms hailo-h10-all || true

# Set the workdir and then copy over the Hailo GenAI model package
WORKDIR /app
COPY hailo_gen_ai_model_zoo_5.1.1_arm64.deb /app/

# Setup the GenAI model package and install the deps
RUN dpkg -i hailo_gen_ai_model_zoo_5.1.1_arm64.deb || \
	apt --fix-broken install -y

RUN rm -f /app/hailo_gen_ai_model_zoo_5.1.1_arm64.deb
