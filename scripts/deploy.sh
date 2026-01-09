#!/usr/bin/env bash
#
# Armiarma Network Crawler - Server Deployment Script
# Target: Fresh Ubuntu 22.04/24.04 LTS
#
# Usage:
#   git clone https://github.com/DecentralizedGeo/armiarma.git
#   cd armiarma && ./scripts/deploy.sh
#
# Safe to run multiple times (idempotent)
#

set -euo pipefail

# ============================================================================
# Configuration
# ============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(dirname "$SCRIPT_DIR")"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step()  { echo -e "${BLUE}[STEP]${NC} $1"; }

# ============================================================================
# Pre-flight Checks
# ============================================================================

preflight_checks() {
    log_step "Running pre-flight checks..."

    # Check OS
    if [[ ! -f /etc/os-release ]]; then
        log_error "Cannot detect OS. This script requires Ubuntu."
        exit 1
    fi

    source /etc/os-release
    if [[ "$ID" != "ubuntu" ]]; then
        log_error "This script is designed for Ubuntu. Detected: $ID"
        exit 1
    fi

    log_info "Detected: $PRETTY_NAME"

    # Check sudo
    if [[ $EUID -ne 0 ]]; then
        if ! sudo -n true 2>/dev/null; then
            log_error "This script requires root or passwordless sudo"
            exit 1
        fi
        SUDO="sudo"
    else
        SUDO=""
        log_warn "Running as root. Consider using a non-root user with sudo."
    fi

    # Check internet
    if ! ping -c 1 -W 5 github.com &>/dev/null; then
        log_error "No internet connectivity to github.com"
        exit 1
    fi

    log_info "Pre-flight checks passed"
}

# ============================================================================
# Install System Dependencies
# ============================================================================

install_dependencies() {
    log_step "Installing system dependencies..."

    $SUDO apt-get update -qq
    $SUDO apt-get install -y -qq \
        curl \
        wget \
        git \
        jq \
        htop \
        tmux \
        fail2ban \
        ufw \
        ca-certificates \
        gnupg \
        lsb-release

    log_info "System dependencies installed"
}

# ============================================================================
# Configure Firewall (SAFE)
# ============================================================================

configure_firewall() {
    log_step "Configuring UFW firewall..."

    # Detect SSH port
    SSH_PORT=$(ss -tlnp | grep sshd | awk '{print $4}' | grep -oE '[0-9]+$' | head -1)
    SSH_PORT="${SSH_PORT:-22}"
    log_info "Detected SSH on port: $SSH_PORT"

    # CRITICAL: Allow SSH FIRST
    $SUDO ufw allow "$SSH_PORT/tcp" comment 'SSH - DO NOT REMOVE'

    # Crawler P2P ports
    $SUDO ufw allow 9020/tcp comment 'Ethereum libp2p'
    $SUDO ufw allow 9020/udp comment 'Ethereum libp2p UDP'
    $SUDO ufw allow 30303/tcp comment 'Polygon/Celo devp2p'
    $SUDO ufw allow 30303/udp comment 'Polygon/Celo devp2p UDP'
    $SUDO ufw allow 30304/tcp comment 'Celo devp2p alt'
    $SUDO ufw allow 30304/udp comment 'Celo devp2p alt UDP'
    $SUDO ufw allow 1347/tcp comment 'Filecoin libp2p'

    # Metrics (localhost only via docker, but open for external monitoring if needed)
    # These are bound to 127.0.0.1 in docker-compose, so external access blocked anyway

    # Set defaults
    $SUDO ufw default deny incoming
    $SUDO ufw default allow outgoing

    # Enable non-interactively
    echo "y" | $SUDO ufw enable || true

    log_info "Firewall configured"
    $SUDO ufw status numbered
}

# ============================================================================
# Install Docker
# ============================================================================

install_docker() {
    if command -v docker &>/dev/null; then
        log_info "Docker already installed: $(docker --version)"
        return
    fi

    log_step "Installing Docker..."

    # Add Docker GPG key
    $SUDO install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | $SUDO gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    $SUDO chmod a+r /etc/apt/keyrings/docker.gpg

    # Add repository
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
      $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
      $SUDO tee /etc/apt/sources.list.d/docker.list > /dev/null

    $SUDO apt-get update -qq
    $SUDO apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

    # Add current user to docker group (if not root)
    if [[ $EUID -ne 0 ]]; then
        $SUDO usermod -aG docker "$USER"
        log_warn "Added $USER to docker group. You may need to log out and back in."
    fi

    $SUDO systemctl enable docker
    $SUDO systemctl start docker

    log_info "Docker installed: $(docker --version)"
}

# ============================================================================
# Install Claude Code (optional)
# ============================================================================

install_claude_code() {
    if command -v claude &>/dev/null; then
        log_info "Claude Code already installed"
        return
    fi

    log_step "Installing Claude Code..."

    # Install Node.js if needed
    if ! command -v node &>/dev/null; then
        curl -fsSL https://deb.nodesource.com/setup_20.x -o /tmp/nodesource_setup.sh
        $SUDO bash /tmp/nodesource_setup.sh
        rm /tmp/nodesource_setup.sh
        $SUDO apt-get install -y -qq nodejs
    fi

    $SUDO npm install -g @anthropic-ai/claude-code

    log_info "Claude Code installed. Run 'claude' to authenticate."
}

# ============================================================================
# Setup Environment Files
# ============================================================================

setup_env_files() {
    log_step "Setting up environment files..."

    cd "$REPO_DIR"

    # Create .env.common if not exists
    if [[ ! -f .env.common ]]; then
        if [[ -f .env_template.common ]]; then
            cp .env_template.common .env.common
            log_info "Created .env.common from template"
        fi
    else
        log_info ".env.common already exists"
    fi

    # Create network-specific env files
    for network in ethereum polygon filecoin celo; do
        if [[ ! -f ".env.${network}" ]]; then
            if [[ -f ".env_template.${network}" ]]; then
                cp ".env_template.${network}" ".env.${network}"
                log_info "Created .env.${network} from template"
            fi
        else
            log_info ".env.${network} already exists"
        fi
    done

    # Fix app-data permissions
    mkdir -p app-data
    chmod 755 app-data
    if [[ -d app-data/postgresql_db ]]; then
        chmod 777 app-data/postgresql_db
    fi
    if [[ -d app-data/prometheus_db ]]; then
        chmod 777 app-data/prometheus_db
    fi
}

# ============================================================================
# Create Systemd Service
# ============================================================================

create_systemd_service() {
    log_step "Creating systemd service..."

    $SUDO tee /etc/systemd/system/armiarma.service > /dev/null << EOF
[Unit]
Description=Armiarma Network Crawler
Requires=docker.service
After=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=$REPO_DIR
ExecStart=/usr/bin/docker compose up -d
ExecStop=/usr/bin/docker compose down
TimeoutStartSec=300

[Install]
WantedBy=multi-user.target
EOF

    $SUDO systemctl daemon-reload
    $SUDO systemctl enable armiarma.service

    log_info "Systemd service 'armiarma' created and enabled"
}

# ============================================================================
# Build and Start Services
# ============================================================================

start_services() {
    log_step "Building and starting services..."

    cd "$REPO_DIR"

    # Need to use sudo if user not in docker group yet
    if groups | grep -q docker; then
        docker compose build
        docker compose up -d
    else
        $SUDO docker compose build
        $SUDO docker compose up -d
    fi

    log_info "Waiting for services to start..."
    sleep 10

    if groups | grep -q docker; then
        docker compose ps
    else
        $SUDO docker compose ps
    fi
}

# ============================================================================
# Print Summary
# ============================================================================

print_summary() {
    local IP
    IP=$(hostname -I | awk '{print $1}')

    echo ""
    echo "============================================================================"
    echo -e "${GREEN}Armiarma Deployment Complete!${NC}"
    echo "============================================================================"
    echo ""
    echo "Server IP: $IP"
    echo "Repo location: $REPO_DIR"
    echo ""
    echo "Services (all bound to localhost - use SSH tunnel to access):"
    echo "  PostgreSQL:  localhost:5432  (user/password)"
    echo "  Prometheus:  localhost:9090"
    echo "  Grafana:     localhost:3000  (admin/admin)"
    echo ""
    echo "Crawler metrics:"
    echo "  Ethereum:  localhost:9080/metrics"
    echo "  Polygon:   localhost:9081/metrics"
    echo "  Filecoin:  localhost:9082/metrics"
    echo "  Celo:      localhost:9083/metrics"
    echo ""
    echo "Access from your local machine:"
    echo "  ssh -L 3000:localhost:3000 -L 9090:localhost:9090 ${USER}@${IP}"
    echo "  Then open: http://localhost:3000"
    echo ""
    echo "Useful commands:"
    echo "  cd $REPO_DIR"
    echo "  docker compose ps              # Check status"
    echo "  docker compose logs -f         # View all logs"
    echo "  docker compose logs -f eth_crawler   # Ethereum logs"
    echo "  docker compose restart         # Restart all"
    echo "  sudo systemctl status armiarma # Systemd status"
    echo ""
    echo "Export data:"
    echo "  ./scripts/export_data.sh --help"
    echo ""
    if command -v claude &>/dev/null; then
        echo "Claude Code: Run 'claude' to authenticate"
        echo ""
    fi
    echo "============================================================================"
}

# ============================================================================
# Main
# ============================================================================

main() {
    echo ""
    echo "============================================================================"
    echo "Armiarma Network Crawler - Deployment Script"
    echo "============================================================================"
    echo ""

    preflight_checks
    install_dependencies
    configure_firewall
    install_docker
    install_claude_code
    setup_env_files
    create_systemd_service
    start_services
    print_summary
}

main "$@"
