#!/usr/bin/env bash
# Cuearr Proxmox LXC installer — source: https://github.com/marcatos/cuearr
# Community-scripts style: run on a Proxmox VE host (creates CT) or inside Debian with --inside.
set -euo pipefail

GITHUB_REPO="marcatos/cuearr"
DATA_DIR="/var/lib/cuearr"
CONFIG_DIR="/etc/cuearr"
BIN="/usr/local/bin/cuearr"
SERVICE="cuearr.service"
HTTP_PORT="8787"

YW="\033[33m"
GN="\033[1;92m"
RD="\033[1;91m"
BL="\033[36m"
CL="\033[m"
CM="  ✔ "
INFO="  💡 "
CROSS="  ✖ "

msg_info() { echo -e "${BL}${INFO}${CL}${YW}$*${CL}" >&2; }
msg_ok() { echo -e "${GN}${CM}${CL}$*" >&2; }
msg_error() { echo -e "${RD}${CROSS}${CL}$*" >&2; }

require_root() {
  if [[ "$(id -u)" -ne 0 ]]; then
    msg_error "Run as root."
    exit 1
  fi
}

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) echo "amd64" ;;
    aarch64 | arm64) echo "arm64" ;;
    *)
      msg_error "Unsupported architecture: $(uname -m)"
      exit 1
      ;;
  esac
}

latest_release_tag() {
  curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" |
    grep -Po '"tag_name"\s*:\s*"\K[^"]+' | head -1
}

install_packages() {
  msg_info "Installing dependencies…"
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y -qq shntool cuetools flac curl ca-certificates tar
  msg_ok "Packages installed"
}

download_binary() {
  local tag arch asset url tmp
  tag="$(latest_release_tag)"
  if [[ -z "${tag}" ]]; then
    msg_error "Could not resolve latest GitHub release for ${GITHUB_REPO}"
    exit 1
  fi
  arch="$(detect_arch)"
  asset="cuearr-${tag}-linux-${arch}.tar.gz"
  url="https://github.com/${GITHUB_REPO}/releases/download/${tag}/${asset}"
  msg_info "Downloading ${url}…"
  tmp="$(mktemp -d)"
  trap 'rm -rf "${tmp}"' RETURN
  curl -fsSL "${url}" -o "${tmp}/${asset}"
  tar -xzf "${tmp}/${asset}" -C "${tmp}"
  install -m 0755 "${tmp}/cuearr-${tag}-linux-${arch}" "${BIN}"
  msg_ok "Installed ${BIN} (${tag})"
}

write_config() {
  mkdir -p "${CONFIG_DIR}" "${DATA_DIR}" /var/watch /var/out
  cat >"${CONFIG_DIR}/cuearr.yaml" <<EOF
http_addr: ":${HTTP_PORT}"
data_dir: ${DATA_DIR}
watch_dirs: ["/var/watch"]
out_dir: "/var/out"
in_place: false
engine: shntool
log_level: info
EOF
  msg_ok "Wrote ${CONFIG_DIR}/cuearr.yaml"
}

install_systemd() {
  if ! id -u cuearr >/dev/null 2>&1; then
    useradd --system --home "${DATA_DIR}" --shell /usr/sbin/nologin cuearr
  fi
  chown -R cuearr:cuearr "${DATA_DIR}" /var/watch /var/out
  cat >/etc/systemd/system/${SERVICE} <<EOF
[Unit]
Description=Cuearr lossless CUE splitter
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=cuearr
Group=cuearr
EnvironmentFile=-/etc/cuearr/cuearr.env
ExecStart=${BIN} serve --config ${CONFIG_DIR}/cuearr.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
  if [[ ! -f /etc/cuearr/cuearr.env ]]; then
    cat >/etc/cuearr/cuearr.env <<'EOF'
# Optional bootstrap (see README): CUEARR_INITIAL_PASSWORD, CUEARR_API_KEY
EOF
    chmod 0600 /etc/cuearr/cuearr.env
  fi
  systemctl daemon-reload
  systemctl enable --now "${SERVICE}"
  msg_ok "systemd unit ${SERVICE} enabled"
}

print_ui_url() {
  local ip
  ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  if [[ -z "${ip}" ]]; then
    ip="<container-ip>"
  fi
  msg_ok "Cuearr UI: http://${ip}:${HTTP_PORT}/"
  msg_info "Set CUEARR_INITIAL_PASSWORD in /etc/cuearr/cuearr.env then: systemctl restart ${SERVICE}"
}

install_inside_ct() {
  require_root
  msg_info "Installing Cuearr inside container…"
  install_packages
  download_binary
  write_config
  install_systemd
  print_ui_url
}

is_pve_host() {
  [[ -f /etc/pve/.version ]]
}

create_lxc() {
  local ctid hostname storage template unprivileged bridge
  ctid="${CTID:-}"
  if [[ -z "${ctid}" ]]; then
    ctid="$(pvesh get /cluster/nextid 2>/dev/null || echo "")"
  fi
  if [[ -z "${ctid}" ]]; then
    read -r -p "Container ID [100]: " ctid
    ctid="${ctid:-100}"
  fi
  hostname="${HOSTNAME_CT:-cuearr}"
  storage="${STORAGE:-local-lvm}"
  template="${TEMPLATE:-local:vztmpl/debian-12-standard_12.7-1_amd64.tar.zst}"
  unprivileged="${UNPRIVILEGED:-1}"
  bridge="${BRIDGE:-vmbr0}"

  msg_info "Creating LXC ${ctid} (${hostname}, unprivileged=${unprivileged})…"
  pct create "${ctid}" "${template}" \
    -hostname "${hostname}" \
    -cores 2 \
    -memory 512 \
    -net0 "name=eth0,bridge=${bridge},ip=dhcp" \
    -rootfs "${storage}:8" \
    -unprivileged "${unprivileged}" \
    -features nesting=0 \
    -onboot 1 >&2
  pct start "${ctid}" >&2
  msg_ok "Container ${ctid} started"
  echo "${ctid}"
}

run_install_in_ct() {
  local ctid="$1"
  local script_path
  script_path="$(readlink -f "$0")"
  pct push "${ctid}" "${script_path}" /tmp/cuearr-install.sh
  pct exec "${ctid}" -- bash /tmp/cuearr-install.sh --inside
  local ip
  ip="$(pct exec "${ctid}" -- hostname -I 2>/dev/null | awk '{print $1}')"
  if [[ -n "${ip}" ]]; then
    msg_ok "Cuearr UI (CT ${ctid}): http://${ip}:${HTTP_PORT}/"
  else
    msg_info "Install finished in CT ${ctid}; check IP with: pct exec ${ctid} -- hostname -I"
  fi
}

main() {
  if [[ "${1:-}" == "--inside" ]]; then
    install_inside_ct
    return
  fi

  if is_pve_host; then
    require_root
    msg_info "Proxmox VE detected — creating Debian LXC and installing Cuearr"
    read -r -p "Privileged LXC? (y/N): " priv
    if [[ "${priv,,}" == "y" ]]; then
      UNPRIVILEGED=0
    else
      UNPRIVILEGED=1
    fi
    ctid="$(create_lxc)"
    run_install_in_ct "${ctid}"
    return
  fi

  install_inside_ct
}

main "$@"
