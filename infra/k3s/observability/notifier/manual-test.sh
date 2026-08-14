#!/usr/bin/env bash
# =============================================================================
# Manual test for the Chinwag Notifier's Discord webhooks.
#
# Sends a test message to each configured category webhook — one per Discord
# chat room:
#   default / incidents / deployments / traffic / recoveries / warnings
#
# Webhook URLs are read from ./.env (or DISCORD_WEBHOOK_URL_* env vars). Fill
# them in first (see .env.example). .env is gitignored, so the real test URLs
# are never committed.
#
# Usage:
#   ./manual-test.sh                  # test every configured webhook (direct)
#   ./manual-test.sh incidents        # test only the incidents webhook
#   ./manual-test.sh --via-notifier   # exercise the notifier routing instead
#                                     # (needs a notifier on NOTIFIER_URL,
#                                     #  default http://localhost:9095)
# =============================================================================
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

# Load local .env so the per-chat-room test webhooks are available.
if [ -f .env ]; then
  set -a; . ./.env; set +a
fi

NOTIFIER_URL="${NOTIFIER_URL:-http://localhost:9095}"
VIA_NOTIFIER=0
[ "${1:-}" = "--via-notifier" ] && VIA_NOTIFIER=1 && shift

categories=(DISCORD_WEBHOOK_URL DISCORD_WEBHOOK_URL_INCIDENTS DISCORD_WEBHOOK_URL_DEPLOYMENTS DISCORD_WEBHOOK_URL_TRAFFIC DISCORD_WEBHOOK_URL_RECOVERIES DISCORD_WEBHOOK_URL_WARNINGS)

# Optional filter: only the given category (default | incidents | ...).
FILTER=""
if [ $# -gt 0 ]; then
  FILTER="DISCORD_WEBHOOK_URL_${1}"
  [ "${1}" = "default" ] && FILTER="DISCORD_WEBHOOK_URL"
fi

for var in "${categories[@]}"; do
  [ -n "${FILTER}" ] && [ "${var}" != "${FILTER}" ] && continue
  url="${!var:-}"
  [ -z "${url}" ] && { echo "==> ${var}: <not configured> (skip)"; continue; }

  label="${var#DISCORD_WEBHOOK_URL_}"
  [ "${var}" = "DISCORD_WEBHOOK_URL" ] && label="default"

  echo "==> ${var} (${label})"

  if [ "${VIA_NOTIFIER}" -eq 1 ]; then
    # Exercise routing: send an Alertmanager payload whose `category` label
    # matches, so the notifier forwards it to the right chat room.
    cat_json=""
    [ "${label}" != "default" ] && cat_json=",\"category\":\"${label}\""
    payload="{\"status\":\"firing\",\"alerts\":[{\"status\":\"firing\",\"labels\":{\"alertname\":\"ManualTest\",\"service\":\"manual\"${cat_json}},\"annotations\":{\"summary\":\"🧪 Manual test for ${label}\"},\"startsAt\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}]}"
    curl -sS -o /dev/null -w "    notifier -> HTTP %{http_code}\n" -X POST \
      -H 'Content-Type: application/json' \
      -d "${payload}" \
      "${NOTIFIER_URL}/webhooks/alertmanager"
  else
    # Direct: post a test embed straight to the category's Discord webhook.
    curl -sS -o /dev/null -w "    discord  -> HTTP %{http_code}\n" -X POST \
      -H 'Content-Type: application/json' \
      -d "{\"content\":\"🧪 Chinwag notifier manual test — **${label}**\",\"embeds\":[{\"title\":\"🧪 Manual test (${label})\",\"description\":\"This confirms the **${label}** chat room webhook is wired correctly.\",\"color\":15953429}]}" \
      "${url}"
  fi
done

echo
echo "Done. Check each Discord chat room for the 🧪 test message."
