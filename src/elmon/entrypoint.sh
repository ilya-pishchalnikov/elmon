#!/bin/sh
set -e

if [ "$ELMON_DEBUG" = "1" ]; then
    echo "ELMON_DEBUG mode - container started but application not running"
    echo "You can exec into container with: docker exec -it <container> /bin/sh"
    echo "Files in /app:" && ls -la /app/
    tail -f /dev/null
else
    exec ./elmon
fi