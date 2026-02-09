#!/usr/bin/env bash

# Glance Installer Script (FrozenDark Edition)
USER_REPO="frozendark01/glance"
INSTALL_PATH="/opt/glance"

echo -e "\e[32m[Info] Initializing Glance (FrozenDark Edition)...\e[0m"

# 1. Architecture Detection
ARCH=$(uname -m)
case $ARCH in
    x86_64) BINARY_ARCH="amd64" ;;
    aarch64) BINARY_ARCH="arm64" ;;
    *) echo -e "\e[31m[Error] Unsupported architecture: $ARCH\e[0m"; exit 1 ;;
esac

# 2. Fetching the latest version from your GitHub repository
RELEASE=$(curl -s https://api.github.com/repos/${USER_REPO}/releases/latest | grep "tag_name" | awk '{print substr($2, 2, length($2)-3)}')

if [ -z "$RELEASE" ]; then
    echo -e "\e[31m[Error] Could not find any version in the repository ${USER_REPO}.\e[0m"
    exit 1
fi

echo -e "\e[34m[Info] Detected version: $RELEASE\e[0m"

# 3. Environment preparation
mkdir -p $INSTALL_PATH
if systemctl is-active --quiet glance; then
    echo -e "\e[33m[Info] Stopping service for update...\e[0m"
    systemctl stop glance
fi

# 4. Download and installation (according to the names in goreleaser.yaml)
URL="https://github.com/${USER_REPO}/releases/download/${RELEASE}/glance-linux-${BINARY_ARCH}.tar.gz"
echo -e "\e[32m[Info] Downloading binary: $URL\e[0m"

if curl -L "$URL" | tar xz -C $INSTALL_PATH glance; then
    chmod +x $INSTALL_PATH/glance
    echo -e "\e[32m[Success] Binary installed at $INSTALL_PATH/glance\e[0m"
else
    echo -e "\e[31m[Error] Download failed!\e[0m"
    exit 1
fi

# 5. Default configuration (if not already present)
if [ ! -f "$INSTALL_PATH/glance.yml" ]; then
    cat <<EOF >$INSTALL_PATH/glance.yml
pages:
  - name: FrozenDark Dashboard
    width: slim
    columns:
      - size: full
        widgets:
          - type: search
          - type: monitors
EOF
fi

# 6. Systemd Service Configuration
cat <<EOF >/etc/systemd/system/glance.service
[Unit]
Description=Glance Dashboard Daemon
After=network.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_PATH
ExecStart=$INSTALL_PATH/glance --config $INSTALL_PATH/glance.yml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# 7. Log Cleanup (every 3 days) - Cron Automation
# We look for logs in the working directory and delete them at midnight every 3 days
if ! crontab -l 2>/dev/null | grep -q "$INSTALL_PATH/*.log"; then
    (crontab -l 2>/dev/null; echo "0 0 */3 * * find $INSTALL_PATH -name '*.log' -delete") | crontab -
    echo -e "\e[32m[Info] Automatic log cleanup set for every 3 days.\e[0m"
fi

# 8. Start Service
systemctl daemon-reload
systemctl enable -q --now glance

echo -e "\e[32m--------------------------------------------------\e[0m"
echo -e "\e[32m[DONE] Glance ${RELEASE} is now running on port 8080!\e[0m"
echo -e "\e[32m--------------------------------------------------\e[0m"