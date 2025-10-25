#!/bin/bash
# Restart Zeus service for RaidHub Services
# This restarts the Zeus service using systemctl

set -e

/usr/bin/systemctl restart zeus

# Log the restart
echo "$(date): Zeus service restarted" >> /RaidHub/cron.log

exit $?
