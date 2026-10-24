#!/usr/bin/env bash
# Build, run and manage Awesome Harnesses as a per-user background service:
# a systemd user unit on Linux, a launchd agent on macOS. `make` wraps these.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$ROOT/bin/server"
NAME=awesome-harness

# Linux: the unit in systemd/ is symlinked into the user's unit directory.
UNIT="$NAME.service"
UNIT_SRC="$ROOT/systemd/$UNIT"
UNIT_DST="$HOME/.config/systemd/user/$UNIT"

# macOS: the agent's plist is generated with this checkout's paths.
LABEL="dev.$NAME"
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
LOG="$HOME/Library/Logs/$NAME.log"
DOMAIN="gui/$(id -u)"

die() { echo "ctl.sh: $*" >&2; exit 1; }

os() {
  case "$(uname -s)" in
    Linux) echo linux ;;
    Darwin) echo mac ;;
    *) die "no service support on $(uname -s); run \`./ctl.sh build && ./ctl.sh run\`" ;;
  esac
}

# load_env exports .env (the server itself doesn't read it).
load_env() {
  if [[ -f "$ROOT/.env" ]]; then
    set -a
    # shellcheck disable=SC1091
    . "$ROOT/.env"
    set +a
  fi
}

url() {
  load_env
  echo "http://${HOST:-127.0.0.1}:${PORT:-8767}"
}

cmd_build() {
  cd "$ROOT"
  pnpm install
  pnpm build
  touch web/dist/.gitkeep
  go build -o "$BIN" ./cmd/server
}

# run starts the server in the foreground with .env applied; the launchd agent runs this too.
cmd_run() {
  [[ -x "$BIN" ]] || die "$BIN is missing; run \`./ctl.sh build\` first"
  load_env
  # launchd may start the agent without SHELL; the terminal panel's shells need the login shell.
  if [[ -z "${SHELL:-}" && "$(uname -s)" == Darwin ]]; then
    SHELL="$(dscl . -read "/Users/$(id -un)" UserShell 2>/dev/null | awk '{print $2}')"
    export SHELL="${SHELL:-/bin/zsh}"
  fi
  cd "$ROOT"
  exec "$BIN"
}

write_plist() {
  mkdir -p "$(dirname "$PLIST")" "$(dirname "$LOG")"
  cat >"$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>$LABEL</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/bash</string>
    <string>$ROOT/ctl.sh</string>
    <string>run</string>
  </array>
  <key>WorkingDirectory</key>
  <string>$ROOT</string>
  <key>RunAtLoad</key>
  <true/>
  <!-- Like systemd's Restart=on-failure: relaunch after a crash, not after a clean exit. -->
  <key>KeepAlive</key>
  <dict>
    <key>SuccessfulExit</key>
    <false/>
  </dict>
  <key>ThrottleInterval</key>
  <integer>3</integer>
  <key>StandardOutPath</key>
  <string>$LOG</string>
  <key>StandardErrorPath</key>
  <string>$LOG</string>
</dict>
</plist>
EOF
}

mac_loaded() { launchctl print "$DOMAIN/$LABEL" >/dev/null 2>&1; }

cmd_install() {
  cmd_build
  [[ -f "$ROOT/.env" ]] || cp "$ROOT/.env.example" "$ROOT/.env"
  case "$(os)" in
    linux)
      mkdir -p "$(dirname "$UNIT_DST")"
      ln -sfn "$UNIT_SRC" "$UNIT_DST"
      systemctl --user daemon-reload
      systemctl --user enable "$UNIT"
      systemctl --user restart "$UNIT"
      systemctl --user --no-pager --full status "$UNIT"
      ;;
    mac)
      write_plist
      if mac_loaded; then launchctl bootout "$DOMAIN/$LABEL" || true; fi
      launchctl bootstrap "$DOMAIN" "$PLIST"
      cmd_status
      ;;
  esac
}

cmd_uninstall() {
  case "$(os)" in
    linux)
      systemctl --user disable --now "$UNIT"
      rm -f "$UNIT_DST"
      systemctl --user daemon-reload
      ;;
    mac)
      if mac_loaded; then launchctl bootout "$DOMAIN/$LABEL"; fi
      rm -f "$PLIST"
      ;;
  esac
}

# up starts the service, installing it first if needed.
cmd_up() {
  case "$(os)" in
    linux)
      [[ -e "$UNIT_DST" ]] || cmd_install
      systemctl --user start "$UNIT"
      ;;
    mac)
      [[ -e "$PLIST" ]] || cmd_install
      # bootstrap loads it (RunAtLoad starts it); kickstart covers a loaded agent that exited.
      if mac_loaded; then launchctl kickstart "$DOMAIN/$LABEL"; else launchctl bootstrap "$DOMAIN" "$PLIST"; fi
      ;;
  esac
  echo "Awesome Harnesses is up: $(url)"
}

# down stops the service; it stays installed (and starts again at the next login).
cmd_down() {
  case "$(os)" in
    linux) systemctl --user stop "$UNIT" ;;
    # Unloading is the only stop launchd won't undo through KeepAlive.
    mac) if mac_loaded; then launchctl bootout "$DOMAIN/$LABEL"; fi ;;
  esac
}

cmd_restart() {
  case "$(os)" in
    linux) systemctl --user restart "$UNIT" ;;
    mac)
      [[ -e "$PLIST" ]] || die "not installed; run \`./ctl.sh install\`"
      if mac_loaded; then launchctl kickstart -k "$DOMAIN/$LABEL"; else launchctl bootstrap "$DOMAIN" "$PLIST"; fi
      ;;
  esac
}

cmd_status() {
  case "$(os)" in
    linux) systemctl --user status "$UNIT" ;;
    mac)
      if ! mac_loaded; then
        [[ -e "$PLIST" ]] && echo "$LABEL: installed, stopped" || echo "$LABEL: not installed"
        return 3
      fi
      launchctl print "$DOMAIN/$LABEL" | grep -E '^[[:space:]]+(state|pid|last exit code) =' | head -n 3 || true
      echo "url = $(url)"
      echo "log = $LOG"
      ;;
  esac
}

cmd_logs() {
  case "$(os)" in
    linux) journalctl --user -u "$UNIT" -f ;;
    mac) touch "$LOG" && tail -n 100 -F "$LOG" ;;
  esac
}

usage() {
  cat <<EOF
usage: ./ctl.sh <command>

  build      build the web bundle and the server binary (bin/server)
  run        run the server in the foreground with .env applied
  install    build, then (re)install and restart the background service
  uninstall  stop the service and remove it
  up         start the service (installs it the first time)
  down       stop the service (it stays installed)
  restart    restart the service
  status     show the service status
  logs       follow the service log

The service is a systemd user unit on Linux and a launchd agent ($LABEL) on macOS.
EOF
}

case "${1:-}" in
  build | run | install | uninstall | up | down | restart | status | logs) "cmd_$1" ;;
  -h | --help | help | "") usage ;;
  *) usage >&2; exit 2 ;;
esac
