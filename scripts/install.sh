#!/bin/bash
#
# Canton Node Health Dashboard - One-Command Install Script
#
# This script installs and configures the complete monitoring stack, including:
#   - Prometheus (for metrics collection)
#   - Grafana (for visualization and dashboards)
#   - canton-node-health-agent (the Go agent to export Canton metrics)
#
# Prerequisites:
#   - Docker
#   - Docker Compose (v1 or v2)
#

set -euo pipefail

# --- Color Codes for Output ---
COLOR_RESET='\033[0m'
COLOR_GREEN='\033[0;32m'
COLOR_YELLOW='\033[0;33m'
COLOR_BLUE='\033[0;34m'
COLOR_RED='\033[0;31m'

# --- Helper Functions ---
log_info() {
    echo -e "${COLOR_BLUE}INFO:${COLOR_RESET} $1"
}

log_success() {
    echo -e "${COLOR_GREEN}SUCCESS:${COLOR_RESET} $1"
}

log_warn() {
    echo -e "${COLOR_YELLOW}WARN:${COLOR_RESET} $1"
}

log_error() {
    echo -e "${COLOR_RED}ERROR:${COLOR_RESET} $1"
    exit 1
}

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# --- Main Script ---

main() {
    log_info "Starting Canton Node Health Dashboard installation..."

    # --- 1. Dependency Checks ---
    log_info "Checking for required dependencies (Docker, Docker Compose)..."
    if ! command_exists docker; then
        log_error "Docker is not installed. Please install Docker before running this script. See: https://docs.docker.com/get-docker/"
    fi

    DOCKER_COMPOSE_CMD="docker-compose"
    if ! command_exists docker-compose; then
      if docker compose version >/dev/null 2>&1; then
        DOCKER_COMPOSE_CMD="docker compose"
        log_info "Detected Docker Compose V2 ('docker compose')."
      else
        log_error "Docker Compose is not installed. Please install it before running this script. See: https://docs.docker.com/compose/install/"
      fi
    else
        log_info "Detected Docker Compose V1 ('docker-compose')."
    fi

    # --- 2. Navigate to Project Root ---
    # This ensures the script can find docker-compose.yml and other config files.
    SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
    PROJECT_ROOT="$SCRIPT_DIR/.."
    cd "$PROJECT_ROOT"
    log_info "Running from project root: $(pwd)"


    # --- 3. Configuration Setup (.env file) ---
    ENV_FILE=".env"
    if [ -f "$ENV_FILE" ]; then
        log_info "Existing '.env' file found. Using existing configuration."
        # shellcheck source=.env
        source "$ENV_FILE"
    else
        log_info "No '.env' file found. Let's create one."
        echo
        echo -e "Please provide the gRPC endpoint for your Canton validator participant node."
        echo -e "This should be the admin API endpoint, typically ending in port 5012 (for TLS) or 5011 (for insecure)."
        echo -e "Example (TLS): ${COLOR_YELLOW}grpc://my-validator.my-domain.com:5012${COLOR_RESET}"
        echo -e "Example (insecure): ${COLOR_YELLOW}http://127.0.0.1:5011${COLOR_RESET}"
        echo

        DEFAULT_CANTON_URL="http://localhost:5011"
        read -r -p "Enter Canton Node Admin API URL [default: $DEFAULT_CANTON_URL]: " CANTON_NODE_GRPC_URL
        CANTON_NODE_GRPC_URL=${CANTON_NODE_GRPC_URL:-$DEFAULT_CANTON_URL}

        # Check if TLS should be enabled based on URL prefix
        CANTON_AGENT_TLS_ENABLED="false"
        if [[ "$CANTON_NODE_GRPC_URL" == "grpc://"* || "$CANTON_NODE_GRPC_URL" == "https://"* ]]; then
          CANTON_AGENT_TLS_ENABLED="true"
        fi

        log_info "Creating '$ENV_FILE' with your configuration..."
        {
            echo "# Canton Node Health Dashboard Environment Configuration"
            echo "# This file is managed by scripts/install.sh"
            echo ""
            echo "# The gRPC Admin API endpoint for the Canton participant node to monitor."
            echo "CANTON_NODE_GRPC_URL=${CANTON_NODE_GRPC_URL}"
            echo ""
            echo "# Set to 'true' if the endpoint uses TLS. Autodetected by install.sh."
            echo "CANTON_AGENT_TLS_ENABLED=${CANTON_AGENT_TLS_ENABLED}"
            echo ""
            echo "# (Optional) Path inside the agent container to a client cert for mTLS."
            echo "# Example: CANTON_AGENT_CLIENT_CERT_PATH=/certs/client.pem"
            echo "# CANTON_AGENT_CLIENT_CERT_PATH="
            echo ""
            echo "# (Optional) Path inside the agent container to a client key for mTLS."
            echo "# Example: CANTON_AGENT_CLIENT_KEY_PATH=/certs/client.key"
            echo "# CANTON_AGENT_CLIENT_KEY_PATH="
            echo ""
            echo "# (Optional) Path inside the agent container to the trusted CA cert for TLS."
            echo "# Example: CANTON_AGENT_CA_CERT_PATH=/certs/ca.pem"
            echo "# CANTON_AGENT_CA_CERT_PATH="

        } > "$ENV_FILE"

        log_success "Configuration saved to '$ENV_FILE'."
    fi

    # --- 4. Start the Monitoring Stack ---
    log_info "Pulling the latest Docker images for Grafana and Prometheus..."
    $DOCKER_COMPOSE_CMD pull

    log_info "Starting the monitoring stack with Docker Compose..."
    # Using -d to run in detached mode.
    # --remove-orphans cleans up any old containers if the service definitions changed.
    $DOCKER_COMPOSE_CMD up --build -d --remove-orphans

    echo
    log_success "Canton Node Health Dashboard has been installed and started!"
    echo

    # --- 5. Post-Installation Instructions ---
    echo -e "------------------------------------------------------------------"
    echo -e "Access your dashboards:"
    echo
    echo -e "  ${COLOR_YELLOW}Grafana:${COLOR_RESET}   http://localhost:3000"
    echo -e "              (Login with user: ${COLOR_GREEN}admin${COLOR_RESET}, password: ${COLOR_GREEN}admin${COLOR_RESET})"
    echo
    echo -e "  ${COLOR_YELLOW}Prometheus:${COLOR_RESET} http://localhost:9090"
    echo
    echo -e "It may take a minute or two for the agent to connect and for metrics to appear in Grafana."
    echo
    echo -e "Useful commands:"
    echo -e "  - To view logs: ${COLOR_GREEN}'$DOCKER_COMPOSE_CMD logs -f'${COLOR_RESET}"
    echo -e "  - To stop the stack: ${COLOR_GREEN}'$DOCKER_COMPOSE_CMD down'${COLOR_RESET}"
    echo -e "  - To restart the stack: ${COLOR_GREEN}'$DOCKER_COMPOSE_CMD restart'${COLOR_RESET}"
    echo
    echo -e "For more details, see the Operator's Guide in the 'docs/' directory."
    echo -e "------------------------------------------------------------------"
}

# --- Execute Main Function ---
main "$@"