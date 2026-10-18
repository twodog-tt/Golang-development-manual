#!/bin/sh
# Run as root via the existing administrative Workbench connection.
# Usage: sh setup.sh /tmp/knowledgebase-deploy.pub /tmp/knowledgebase-receive.py
set -eu
test "$(id -u)" = 0
test "$#" = 2
public_key=$(cat "$1")
printf '%s\n' "$public_key" | grep -Eq '^ssh-ed25519 [A-Za-z0-9+/=]+( [A-Za-z0-9@._-]+)?$'
if ! id kbdeploy >/dev/null 2>&1; then
    useradd --system --create-home --home-dir /var/lib/knowledgebase-deploy \
        --shell /bin/sh --comment 'Knowledgebase GitHub Actions deployment' kbdeploy
fi
test "$(getent passwd kbdeploy | cut -d: -f6)" = /var/lib/knowledgebase-deploy
install -d -o root -g root -m 755 /var/lib/knowledgebase-deploy /var/lib/knowledgebase-deploy/.ssh
install -d -o root -g root -m 755 /usr/local/lib/knowledgebase
install -o root -g root -m 644 "$2" /usr/local/lib/knowledgebase/receive.py
printf 'restrict,command="/usr/bin/python3 /usr/local/lib/knowledgebase/receive.py" %s\n' "$public_key" \
    > /var/lib/knowledgebase-deploy/.ssh/authorized_keys
chown root:root /var/lib/knowledgebase-deploy/.ssh/authorized_keys
chmod 644 /var/lib/knowledgebase-deploy/.ssh/authorized_keys
install -d -o kbdeploy -g kbdeploy -m 755 /var/www/knowledgebase /var/www/knowledgebase/releases
sshd -t
printf 'Restricted deployment account installed. SSH host key:\n'
cat /etc/ssh/ssh_host_ed25519_key.pub
