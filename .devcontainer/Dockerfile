FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive

# Installations that require additional steps or are not available via apt
# Azure CLI
RUN curl -sL https://aka.ms/InstallAzureCLIDeb | bash
# TODO: Figure out if we need these
# Docker
# RUN curl -fsSL https://get.docker.com -o get-docker.sh && sh get-docker.sh
# Docker Compose
# RUN curl -L "https://github.com/docker/compose/releases/download/1.29.2/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose && chmod +x /usr/local/bin/docker-compose

# Make Zsh the default shell
ENV SHELL=/bin/zsh

# Create a non-root user 'devuser'
RUN useradd -m -s /bin/zsh devuser

# Set the working directory to home directory of 'devuser'
WORKDIR /home/devuser/src

# Copy the current directory contents into the container at /home/devuser/src
# Assuming Dockerfile is inside .devcontainer directory
COPY ../. /home/devuser/src

# Change ownership of the /home/devuser/src directory to 'devuser'
RUN chown -R devuser:devuser /home/devuser/src

# Switch to the new user 'devuser'
USER devuser

# Set the command to start Zsh
CMD ["zsh"]
