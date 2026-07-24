#!/bin/sh
set -e

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

INSTALL_DIR="${INSTALL_DIR:-/opt/xboard/agent}"
SKIP_SYSTEMD="${XBOARD_INSTALL_SKIP_SYSTEMD:-0}"

USER_XBOARD_RELEASE_REPO="${XBOARD_RELEASE_REPO:-}"
USER_XBOARD_RELEASE_TAG="${XBOARD_RELEASE_TAG:-}"
USER_XBOARD_RELEASE_BASE_URL="${XBOARD_RELEASE_BASE_URL:-}"

DEFAULT_XBOARD_RELEASE_REPO="${USER_XBOARD_RELEASE_REPO:-creamcroissant/xboard2p}"
DEFAULT_XBOARD_RELEASE_TAG="${USER_XBOARD_RELEASE_TAG:-latest}"
DEFAULT_XBOARD_RELEASE_BASE_URL="${USER_XBOARD_RELEASE_BASE_URL:-https://github.com}"
XBOARD_RELEASE_REPO="$DEFAULT_XBOARD_RELEASE_REPO"
XBOARD_RELEASE_TAG="$DEFAULT_XBOARD_RELEASE_TAG"
XBOARD_RELEASE_BASE_URL="$DEFAULT_XBOARD_RELEASE_BASE_URL"
# Deprecated compatibility flag. Release download is strict-only now.
: "${XBOARD_RELEASE_DOWNLOAD_STRICT:=1}"

XBOARD_BOOTSTRAP_REPO="${XBOARD_BOOTSTRAP_REPO:-creamcroissant/xboard2p}"
XBOARD_BOOTSTRAP_REF="${XBOARD_BOOTSTRAP_REF:-latest}"
XBOARD_BOOTSTRAP_RAW_BASE_URL="${XBOARD_BOOTSTRAP_RAW_BASE_URL:-https://raw.githubusercontent.com}"
XBOARD_BOOTSTRAP_RELEASE_BASE_URL="${XBOARD_BOOTSTRAP_RELEASE_BASE_URL:-https://github.com}"
XBOARD_BOOTSTRAP_API_BASE_URL="${XBOARD_BOOTSTRAP_API_BASE_URL:-https://api.github.com}"
XBOARD_AGENT_SCRIPT_URL="${XBOARD_AGENT_SCRIPT_URL:-}"
XBOARD_AGENT_SERVICE_URL="${XBOARD_AGENT_SERVICE_URL:-}"
XBOARD_BOOTSTRAP_KEEP_TEMP="${XBOARD_BOOTSTRAP_KEEP_TEMP:-0}"
# Deprecated compatibility flag. Bootstrap is strict-only now.
: "${XBOARD_BOOTSTRAP_DOWNLOAD_STRICT:=1}"

XBOARD_CORE_RELEASE_API_BASE_URL="${XBOARD_CORE_RELEASE_API_BASE_URL:-https://api.github.com}"
XBOARD_CORE_INSTALL_BASE_DIR="${XBOARD_CORE_INSTALL_BASE_DIR:-${INSTALL_DIR}/cores}"
XBOARD_SINGBOX_RELEASE_REPO="${XBOARD_SINGBOX_RELEASE_REPO:-SagerNet/sing-box}"
XBOARD_SINGBOX_V2RAY_RELEASE_REPO="${XBOARD_SINGBOX_V2RAY_RELEASE_REPO:-creamcroissant/sing-box_with_api}"
XBOARD_XRAY_RELEASE_REPO="${XBOARD_XRAY_RELEASE_REPO:-XTLS/Xray-core}"
XBOARD_SINGBOX_BINARY_PATH="${XBOARD_SINGBOX_BINARY_PATH:-${INSTALL_DIR}/bin/sing-box}"
XBOARD_XRAY_BINARY_PATH="${XBOARD_XRAY_BINARY_PATH:-${INSTALL_DIR}/bin/xray}"
XBOARD_AGENT_CORE_OUTPUT="${XBOARD_AGENT_CORE_OUTPUT:-}"

OS_RAW=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS_RAW" in
    linux*) OS="linux" ;;
    darwin*) OS="darwin" ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *) OS="$OS_RAW" ;;
esac

ARCH_RAW=$(uname -m)
case "$ARCH_RAW" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) ARCH="$ARCH_RAW" ;;
esac

DISTRO_ID=""
DISTRO_ID_LIKE=""
PKG_MANAGER=""
PKG_CACHE_UPDATED=0
OPENRC_SERVICE_CMD=""
OPENRC_UPDATE_CMD=""
CURRENT_STAGE="startup"
XBOARD_INSTALL_SKIP_CONNECT_CHECK="${XBOARD_INSTALL_SKIP_CONNECT_CHECK:-0}"

set_stage() {
    CURRENT_STAGE=$1
    echo "Stage: ${CURRENT_STAGE}"
}

print_failure_context() {
    echo "Install failed at stage: ${CURRENT_STAGE:-unknown}."
    echo "Install dir: ${INSTALL_DIR}"
    echo "Release: repo=${XBOARD_RELEASE_REPO} tag=${XBOARD_RELEASE_TAG} os=${OS} arch=${ARCH}"
    echo "Log hints: systemd -> journalctl -u xboard-agent -n 100 --no-pager"
    echo "Log hints: OpenRC -> rc-service xboard-agent status; check /var/log/messages or /var/log/syslog"
}

fail_stage() {
    echo "Failure: $1"
    print_failure_context
    exit 1
}

command_status() {
    cmd_name=$1
    if [ "$cmd_name" = "ca-certificates" ]; then
        if has_ca_certificates; then
            printf '%s' "available"
        else
            printf '%s' "missing"
        fi
        return 0
    fi

    if command -v "$cmd_name" >/dev/null 2>&1; then
        printf '%s' "available"
    else
        printf '%s' "missing"
    fi
}

print_required_command_summary() {
    echo "Required commands:"
    for cmd_name in "$@"; do
        echo "  ${cmd_name}: $(command_status "$cmd_name")"
    done
}

detect_init_label() {
    if [ "$SKIP_SYSTEMD" = "1" ]; then
        printf '%s' "service install skipped"
        return 0
    fi

    if is_systemd_available; then
        printf '%s' "systemd"
        return 0
    fi

    if is_openrc_available; then
        printf '%s' "openrc (${OPENRC_SERVICE_CMD}, ${OPENRC_UPDATE_CMD})"
        return 0
    fi

    printf '%s' "none detected"
}

print_agent_install_summary() {
    load_os_release
    detect_pkg_manager >/dev/null 2>&1 || true

    distro="${DISTRO_ID:-unknown}"
    if [ -n "$DISTRO_ID_LIKE" ]; then
        distro="${distro} (like ${DISTRO_ID_LIKE})"
    fi

    agent_asset="agent-${OS}-${ARCH}"
    if [ "$OS" = "windows" ]; then
        agent_asset="agent-${OS}-${ARCH}.exe"
    fi

    required_commands="curl ca-certificates unzip"
    if [ -n "$WITH_CORE_TYPE" ] || [ -n "$CORE_ACTION" ]; then
        required_commands="${required_commands} python3 tar"
    fi

    echo "Install summary:"
    echo "  component: agent"
    echo "  install_dir: ${INSTALL_DIR}"
    echo "  os/arch: ${OS}/${ARCH}"
    echo "  distro: ${distro}"
    echo "  init: $(detect_init_label)"
    echo "  package_manager: ${PKG_MANAGER:-not detected}"
    echo "  release: repo=${XBOARD_RELEASE_REPO} tag=${XBOARD_RELEASE_TAG} base=${XBOARD_RELEASE_BASE_URL}"
    echo "  release_asset: ${agent_asset}"
    echo "  grpc_address: ${GRPC_ADDRESS:-not configured}"
    echo "  auth: shared first registration token; host token is written back after first boot"
    if [ -n "$WITH_CORE_TYPE" ]; then
        core_ref="$CORE_VERSION"
        if [ -z "$core_ref" ]; then
            core_ref="${CORE_CHANNEL:-latest}"
        fi
        echo "  bundled_core: type=${WITH_CORE_TYPE} ref=${core_ref} flavor=${CORE_FLAVOR:-official}"
    fi
    if [ -n "$CORE_ACTION" ]; then
        core_ref="$CORE_VERSION"
        if [ -z "$core_ref" ]; then
            core_ref="${CORE_CHANNEL:-latest}"
        fi
        echo "  core_action: action=${CORE_ACTION} type=${CORE_TYPE} ref=${core_ref} flavor=${CORE_FLAVOR:-official}"
    fi
    print_required_command_summary $required_commands
}

extract_host_port() {
    address_value=$1
    case "$address_value" in
        *://*) address_value=${address_value#*://} ;;
    esac
    address_value=${address_value%%/*}
    port_value=${address_value##*:}
    host_value=${address_value%:*}
    case "$host_value" in
        \[*\])
            host_value=${host_value#\[}
            host_value=${host_value%\]}
            ;;
    esac

    case "$port_value" in
        ""|*[!0-9]*)
            return 1
            ;;
    esac

    if [ -z "$host_value" ] || [ "$host_value" = "$port_value" ]; then
        return 1
    fi

    printf '%s %s' "$host_value" "$port_value"
    return 0
}

print_agent_connectivity_summary() {
    echo "Panel connectivity preflight (warning only):"
    if [ -z "$GRPC_ADDRESS" ]; then
        echo "  grpc: skipped, address is not configured yet"
        return 0
    fi

    host_port=$(extract_host_port "$GRPC_ADDRESS" || true)
    if [ -z "$host_port" ]; then
        echo "  grpc: skipped, unable to parse ${GRPC_ADDRESS}"
        return 0
    fi

    set -- $host_port
    target_host=$1
    target_port=$2

    if [ "$XBOARD_INSTALL_SKIP_CONNECT_CHECK" = "1" ]; then
        echo "  grpc: skipped (XBOARD_INSTALL_SKIP_CONNECT_CHECK=1)"
        return 0
    fi

    if command -v nc >/dev/null 2>&1; then
        if nc -z -w 2 "$target_host" "$target_port" >/dev/null 2>&1; then
            echo "  grpc: ${target_host}:${target_port} is reachable"
        else
            echo "  grpc: warning, ${target_host}:${target_port} is not reachable from this host"
        fi
    else
        echo "  grpc: skipped, install nc/netcat for connectivity diagnostics"
    fi
}

agent_config_inputs_required() {
    config_path="${INSTALL_DIR}/config.yml"
    legacy_config_path="${INSTALL_DIR}/agent_config.yml"

    if [ "$FORCE_CONFIG_OVERWRITE" = "1" ]; then
        return 0
    fi

    if [ -f "$config_path" ] || [ -f "$legacy_config_path" ]; then
        return 1
    fi

    return 0
}

validate_agent_install_inputs() {
    if ! agent_config_inputs_required; then
        return 0
    fi

    if [ -z "$GRPC_ADDRESS" ]; then
        echo "Error: missing required config parameters."
        echo "grpc address is required to initialize config.yml."
        echo "Example:"
        echo "  sh ./deploy/agent.sh -k '<communication-key>' -g '127.0.0.1:9090'"
        return 1
    fi

    if [ -z "$COMMUNICATION_KEY" ]; then
        echo "Error: missing required authentication parameters."
        echo "communication_key is required to initialize config.yml."
        echo "host_token can only be written back by the Agent after first-boot registration."
        return 1
    fi

    return 0
}

strip_quotes() {
    value=$1
    value=${value#\"}
    value=${value%\"}
    value=${value#\'}
    value=${value%\'}
    printf '%s' "$value"
}

load_os_release() {
    if [ -n "$DISTRO_ID" ] || [ -n "$DISTRO_ID_LIKE" ]; then
        return 0
    fi

    if [ ! -r /etc/os-release ]; then
        return 0
    fi

    while IFS='=' read -r key value; do
        case "$key" in
            ID)
                DISTRO_ID=$(strip_quotes "$value")
                ;;
            ID_LIKE)
                DISTRO_ID_LIKE=$(strip_quotes "$value")
                ;;
        esac
    done < /etc/os-release

    DISTRO_ID=$(printf '%s' "$DISTRO_ID" | tr '[:upper:]' '[:lower:]')
    DISTRO_ID_LIKE=$(printf '%s' "$DISTRO_ID_LIKE" | tr '[:upper:]' '[:lower:]')
}

has_like() {
    like_word=$1
    case " $DISTRO_ID_LIKE " in
        *" $like_word "*)
            return 0
            ;;
    esac
    return 1
}

detect_pkg_manager() {
    if [ -n "$PKG_MANAGER" ]; then
        return 0
    fi

    load_os_release

    case "$DISTRO_ID" in
        ubuntu|debian)
            if command -v apt-get >/dev/null 2>&1; then
                PKG_MANAGER="apt-get"
                return 0
            fi
            ;;
        fedora)
            if command -v dnf >/dev/null 2>&1; then
                PKG_MANAGER="dnf"
                return 0
            fi
            ;;
        rhel|rocky|almalinux|ol|amzn|centos)
            if command -v dnf >/dev/null 2>&1; then
                PKG_MANAGER="dnf"
                return 0
            fi
            if command -v yum >/dev/null 2>&1; then
                PKG_MANAGER="yum"
                return 0
            fi
            ;;
        alpine)
            if command -v apk >/dev/null 2>&1; then
                PKG_MANAGER="apk"
                return 0
            fi
            ;;
        opensuse*|sles|sled)
            if command -v zypper >/dev/null 2>&1; then
                PKG_MANAGER="zypper"
                return 0
            fi
            ;;
        arch|manjaro)
            if command -v pacman >/dev/null 2>&1; then
                PKG_MANAGER="pacman"
                return 0
            fi
            ;;
    esac

    if has_like "debian" && command -v apt-get >/dev/null 2>&1; then
        PKG_MANAGER="apt-get"
        return 0
    fi

    if has_like "rhel" || has_like "fedora"; then
        if command -v dnf >/dev/null 2>&1; then
            PKG_MANAGER="dnf"
            return 0
        fi
        if command -v yum >/dev/null 2>&1; then
            PKG_MANAGER="yum"
            return 0
        fi
    fi

    if has_like "suse" && command -v zypper >/dev/null 2>&1; then
        PKG_MANAGER="zypper"
        return 0
    fi

    if has_like "arch" && command -v pacman >/dev/null 2>&1; then
        PKG_MANAGER="pacman"
        return 0
    fi

    for manager in apt-get dnf yum apk zypper pacman; do
        if command -v "$manager" >/dev/null 2>&1; then
            PKG_MANAGER="$manager"
            return 0
        fi
    done

    PKG_MANAGER=""
    return 1
}

pkg_manager_env_key() {
    printf '%s' "$1" | tr '[:lower:]-' '[:upper:]_'
}

dependency_package_name() {
    dep_name=$1
    manager=$2

    dep_key=$(printf '%s' "$dep_name" | tr '[:lower:]-' '[:upper:]_')
    manager_key=$(pkg_manager_env_key "$manager")

    eval "override_pkg=\${XBOARD_PKG_${dep_key}_${manager_key}:-}"
    if [ -z "$override_pkg" ]; then
        eval "override_pkg=\${XBOARD_PKG_${dep_key}:-}"
    fi

    if [ -n "$override_pkg" ]; then
        printf '%s' "$override_pkg"
        return 0
    fi

    case "$dep_name" in
        curl)
            printf '%s' "curl"
            ;;
        ca-certificates)
            printf '%s' "ca-certificates"
            ;;
        python3)
            printf '%s' "python3"
            ;;
        *)
            printf '%s' "$dep_name"
            ;;
    esac
}

install_packages() {
    if [ "$#" -eq 0 ]; then
        return 0
    fi

    if ! detect_pkg_manager; then
        echo "Error: no supported package manager detected."
        echo "Please manually install required dependencies: $*"
        return 1
    fi

    echo "Installing packages via ${PKG_MANAGER}: $*"

    case "$PKG_MANAGER" in
        apt-get)
            if [ "$PKG_CACHE_UPDATED" != "1" ]; then
                if ! run_privileged apt-get update; then
                    return 1
                fi
                PKG_CACHE_UPDATED=1
            fi
            run_privileged env DEBIAN_FRONTEND=noninteractive apt-get install -y "$@"
            ;;
        dnf)
            run_privileged dnf install -y "$@"
            ;;
        yum)
            run_privileged yum install -y "$@"
            ;;
        apk)
            run_privileged apk add --no-cache "$@"
            ;;
        zypper)
            run_privileged zypper --non-interactive install "$@"
            ;;
        pacman)
            run_privileged pacman -Sy --noconfirm --needed "$@"
            ;;
        *)
            echo "Error: unsupported package manager: ${PKG_MANAGER}"
            return 1
            ;;
    esac
}

dependency_available() {
    dep_name=$1

    case "$dep_name" in
        curl)
            command -v curl >/dev/null 2>&1
            ;;
        ca-certificates)
            has_ca_certificates
            ;;
        *)
            command -v "$dep_name" >/dev/null 2>&1
            ;;
    esac
}

manual_dependency_hint() {
    dep_name=$1

    case "$dep_name" in
        ca-certificates)
            printf '%s' "ca-certificates"
            ;;
        *)
            printf '%s' "$dep_name"
            ;;
    esac
}

ensure_dependency() {
    dep_name=$1

    if dependency_available "$dep_name"; then
        return 0
    fi

    if ! detect_pkg_manager; then
        echo "Error: dependency '${dep_name}' is missing and no supported package manager was detected."
        echo "Please manually install: $(manual_dependency_hint "$dep_name")."
        return 1
    fi

    pkg_name=$(dependency_package_name "$dep_name" "$PKG_MANAGER")
    if [ -z "$pkg_name" ]; then
        echo "Error: failed to resolve package name for dependency '${dep_name}'."
        return 1
    fi

    if ! install_packages "$pkg_name"; then
        echo "Error: failed to install dependency '${dep_name}' (package: ${pkg_name})."
        echo "Please manually install: $(manual_dependency_hint "$dep_name")."
        return 1
    fi

    if dependency_available "$dep_name"; then
        return 0
    fi

    echo "Error: dependency '${dep_name}' is still unavailable after installation."
    return 1
}

ensure_download_dependencies() {
    if ! ensure_dependency "curl"; then
        return 1
    fi

    if ! ensure_dependency "ca-certificates"; then
        return 1
    fi

    if ! ensure_dependency "unzip"; then
        return 1
    fi

    return 0
}

run_privileged() {
    if [ "$(id -u)" -eq 0 ]; then
        "$@"
        return $?
    fi

    if command -v sudo >/dev/null 2>&1; then
        sudo "$@"
        return $?
    fi

    echo "Error: root privileges are required to run: $*"
    echo "Please run as root or install sudo."
    return 1
}

ensure_dir() {
    dir_path=$1

    if mkdir -p "$dir_path" >/dev/null 2>&1; then
        return 0
    fi

    parent_dir=$(dirname "$dir_path")
    if [ -n "$parent_dir" ] && [ -w "$parent_dir" ]; then
        mkdir -p "$dir_path"
        return $?
    fi

    run_privileged mkdir -p "$dir_path"
}

install_file() {
    src_path=$1
    dst_path=$2

    # Remove destination first so running binaries can be replaced.
    rm -f "$dst_path" 2>/dev/null

    if cp "$src_path" "$dst_path" >/dev/null 2>&1; then
        return 0
    fi

    run_privileged rm -f "$dst_path" 2>/dev/null
    run_privileged cp "$src_path" "$dst_path"
}

set_file_mode() {
    mode_value=$1
    target_path=$2

    if chmod "$mode_value" "$target_path" >/dev/null 2>&1; then
        return 0
    fi

    run_privileged chmod "$mode_value" "$target_path"
}

install_executable_file() {
    src_path=$1
    dst_path=$2

    if ! install_file "$src_path" "$dst_path"; then
        echo "Error: failed to copy file to ${dst_path}."
        return 1
    fi

    if ! set_file_mode +x "$dst_path"; then
        echo "Error: failed to set executable permission on ${dst_path}."
        return 1
    fi

    return 0
}

has_ca_certificates() {
    if [ -f /etc/ssl/certs/ca-certificates.crt ]; then
        return 0
    fi

    if [ -f /etc/ssl/cert.pem ]; then
        return 0
    fi

    if [ -f /etc/pki/tls/certs/ca-bundle.crt ]; then
        return 0
    fi

    if [ -f /etc/ssl/ca-bundle.pem ]; then
        return 0
    fi

    return 1
}

ensure_install_dir() {
    if ! ensure_dir "$INSTALL_DIR"; then
        echo "Error: cannot create install directory ${INSTALL_DIR}."
        return 1
    fi

    return 0
}

is_systemd_available() {
    if ! command -v systemctl >/dev/null 2>&1; then
        return 1
    fi

    if [ ! -d /run/systemd/system ]; then
        return 1
    fi

    if ! systemctl --version >/dev/null 2>&1; then
        return 1
    fi

    return 0
}

resolve_openrc_commands() {
    if [ -n "$OPENRC_SERVICE_CMD" ] && [ -n "$OPENRC_UPDATE_CMD" ]; then
        return 0
    fi

    OPENRC_SERVICE_CMD=""
    OPENRC_UPDATE_CMD=""

    for candidate in rc-service /sbin/rc-service /usr/sbin/rc-service; do
        if [ "$candidate" = "rc-service" ]; then
            if command -v rc-service >/dev/null 2>&1; then
                OPENRC_SERVICE_CMD=$(command -v rc-service)
                break
            fi
        elif [ -x "$candidate" ]; then
            OPENRC_SERVICE_CMD="$candidate"
            break
        fi
    done

    for candidate in rc-update /sbin/rc-update /usr/sbin/rc-update; do
        if [ "$candidate" = "rc-update" ]; then
            if command -v rc-update >/dev/null 2>&1; then
                OPENRC_UPDATE_CMD=$(command -v rc-update)
                break
            fi
        elif [ -x "$candidate" ]; then
            OPENRC_UPDATE_CMD="$candidate"
            break
        fi
    done

    if [ -n "$OPENRC_SERVICE_CMD" ] && [ -n "$OPENRC_UPDATE_CMD" ]; then
        return 0
    fi

    return 1
}

is_openrc_available() {
    if [ "$SKIP_SYSTEMD" = "1" ]; then
        return 1
    fi

    if ! resolve_openrc_commands; then
        return 1
    fi
    return 0
}

render_install_service_file() {
    source_path=$1
    target_path=$2

    temp_service=$(mktemp)
    if [ -z "$temp_service" ]; then
        echo "Error: failed to create temporary service file."
        return 1
    fi

    install_root=$(dirname "$INSTALL_DIR")
    install_dir_placeholder="__XBOARD_INSTALL_DIR__"
    escaped_install_dir=$(printf '%s' "$INSTALL_DIR" | sed 's/[&#\\]/\\&/g')
    escaped_install_root=$(printf '%s' "$install_root" | sed 's/[&#\\]/\\&/g')
    if ! sed -e "s#/opt/xboard/agent#${install_dir_placeholder}#g" -e "s#/opt/xboard#${escaped_install_root}#g" -e "s#${install_dir_placeholder}#${escaped_install_dir}#g" "$source_path" > "$temp_service"; then
        echo "Error: failed to render service file ${source_path}."
        rm -f "$temp_service"
        return 1
    fi

    if ! run_privileged cp "$temp_service" "$target_path"; then
        echo "Error: failed to install rendered service file ${target_path}."
        rm -f "$temp_service"
        return 1
    fi

    rm -f "$temp_service"
    return 0
}

install_openrc_service() {
    service_name=$1
    binary_path=$2
    command_args=$3

    init_script_path="/etc/init.d/${service_name}"
    temp_script=$(mktemp)
    if [ -z "$temp_script" ]; then
        echo "Error: failed to create temporary OpenRC service file."
        return 1
    fi

    cat > "$temp_script" <<EOF
#!/sbin/openrc-run
name="${service_name}"
description="xboard ${service_name} service"
directory="${INSTALL_DIR}"
command="${binary_path}"
command_args="${command_args}"
pidfile="/run/${service_name}.pid"
command_background=true

depend() {
    need net
}
EOF

    if ! run_privileged cp "$temp_script" "$init_script_path"; then
        echo "Error: failed to install OpenRC service script ${init_script_path}."
        rm -f "$temp_script"
        return 1
    fi

    if ! run_privileged chmod +x "$init_script_path"; then
        echo "Error: failed to set executable permission on ${init_script_path}."
        rm -f "$temp_script"
        return 1
    fi

    rm -f "$temp_script"

    if ! run_privileged "$OPENRC_UPDATE_CMD" add "$service_name" default; then
        echo "Error: failed to register OpenRC service ${service_name}."
        return 1
    fi

    if ! run_privileged "$OPENRC_SERVICE_CMD" "$service_name" start; then
        echo "Error: failed to start OpenRC service ${service_name}."
        return 1
    fi

    echo "${service_name} OpenRC service installed."
    return 0
}

download_release_binary() {
    bin_name=$1
    target_path=$2

    ext=""
    if [ "$OS" = "windows" ]; then
        ext=".exe"
    fi

    asset="${bin_name}-${OS}-${ARCH}${ext}"
    base="${XBOARD_RELEASE_BASE_URL%/}"

    if [ "$XBOARD_RELEASE_TAG" = "latest" ]; then
        url="${base}/${XBOARD_RELEASE_REPO}/releases/latest/download/${asset}"
        checksum_url="${base}/${XBOARD_RELEASE_REPO}/releases/latest/download/SHA256SUMS.txt"
    else
        url="${base}/${XBOARD_RELEASE_REPO}/releases/download/${XBOARD_RELEASE_TAG}/${asset}"
        checksum_url="${base}/${XBOARD_RELEASE_REPO}/releases/download/${XBOARD_RELEASE_TAG}/SHA256SUMS.txt"
    fi

    echo "Release asset: ${asset}"
    echo "Release source: repo=${XBOARD_RELEASE_REPO} tag=${XBOARD_RELEASE_TAG} url=${url}"

    if ! command -v curl >/dev/null 2>&1; then
        echo "Error: curl not found for release download of ${bin_name}."
        echo "repo=${XBOARD_RELEASE_REPO} tag=${XBOARD_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        return 1
    fi

    if ! has_ca_certificates; then
        echo "Error: CA certificates not found for release download of ${bin_name}."
        echo "repo=${XBOARD_RELEASE_REPO} tag=${XBOARD_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        return 1
    fi

    tmp_bin=$(mktemp)
    if [ -z "$tmp_bin" ]; then
        echo "Error: failed to create temporary file for ${asset}."
        echo "repo=${XBOARD_RELEASE_REPO} tag=${XBOARD_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        return 1
    fi

    tmp_checksums=$(mktemp)
    if [ -z "$tmp_checksums" ]; then
        echo "Error: failed to create temporary file for checksum manifest."
        rm -f "$tmp_bin"
        return 1
    fi

    if ! curl --fail --silent --show-error --location --retry 3 --retry-delay 1 --output "$tmp_bin" "$url"; then
        echo "Error: failed to download release asset ${asset}."
        echo "repo=${XBOARD_RELEASE_REPO} tag=${XBOARD_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        rm -f "$tmp_bin" "$tmp_checksums"
        return 1
    fi

    if [ ! -s "$tmp_bin" ]; then
        echo "Error: downloaded ${asset} is empty."
        echo "repo=${XBOARD_RELEASE_REPO} tag=${XBOARD_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        rm -f "$tmp_bin" "$tmp_checksums"
        return 1
    fi

    if ! curl --fail --silent --show-error --location --retry 3 --retry-delay 1 --output "$tmp_checksums" "$checksum_url"; then
        echo "Error: failed to download checksum manifest SHA256SUMS.txt."
        echo "repo=${XBOARD_RELEASE_REPO} tag=${XBOARD_RELEASE_TAG} os=${OS} arch=${ARCH} checksum_url=${checksum_url}"
        rm -f "$tmp_bin" "$tmp_checksums"
        return 1
    fi

    if [ ! -s "$tmp_checksums" ]; then
        echo "Error: downloaded checksum manifest is empty."
        echo "repo=${XBOARD_RELEASE_REPO} tag=${XBOARD_RELEASE_TAG} checksum_url=${checksum_url}"
        rm -f "$tmp_bin" "$tmp_checksums"
        return 1
    fi

    if ! verify_checksum "$asset" "$tmp_bin" "$tmp_checksums"; then
        echo "Error: checksum verification failed for release asset ${asset}."
        echo "repo=${XBOARD_RELEASE_REPO} tag=${XBOARD_RELEASE_TAG} checksum_url=${checksum_url}"
        rm -f "$tmp_bin" "$tmp_checksums"
        return 1
    fi

    if ! install_executable_file "$tmp_bin" "$target_path"; then
        echo "Error: failed to install ${asset} into ${target_path}."
        echo "repo=${XBOARD_RELEASE_REPO} tag=${XBOARD_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        rm -f "$tmp_bin" "$tmp_checksums"
        return 1
    fi

    rm -f "$tmp_bin" "$tmp_checksums"
    echo "Installed ${bin_name} from release asset: ${url}"
    return 0
}

fetch_release_metadata() {
    repo=$1
    version=$2
    output_path=$3

    api_base="${XBOARD_CORE_RELEASE_API_BASE_URL%/}"
    if [ -z "$repo" ]; then
        echo "Error: release repository is required."
        return 1
    fi

    if [ "$version" = "" ] || [ "$version" = "latest" ]; then
        url="${api_base}/repos/${repo}/releases/latest"
    else
        normalized_version=$version
        case "$normalized_version" in
            v*) ;;
            *) normalized_version="v${normalized_version}" ;;
        esac
        url="${api_base}/repos/${repo}/releases/tags/${normalized_version}"
    fi

    if ! download_file "$url" "$output_path" "release metadata"; then
        return 1
    fi
    return 0
}

json_get_release_tag() {
    metadata_file=$1
    python3 - "$metadata_file" <<'PY'
import json, sys
with open(sys.argv[1], 'r', encoding='utf-8') as f:
    data = json.load(f)
print(data.get('tag_name', ''))
PY
}

json_get_asset_field() {
    metadata_file=$1
    asset_name=$2
    field_name=$3
    python3 - "$metadata_file" "$asset_name" "$field_name" <<'PY'
import json, sys
with open(sys.argv[1], 'r', encoding='utf-8') as f:
    data = json.load(f)
asset_name = sys.argv[2]
field_name = sys.argv[3]
for asset in data.get('assets', []):
    if asset.get('name') == asset_name:
        value = asset.get(field_name, '')
        if value is None:
            value = ''
        print(value)
        break
PY
}

core_output_is_json() {
    [ "$XBOARD_AGENT_CORE_OUTPUT" = "json" ]
}

emit_core_install_success() {
    core_type=$1
    action=$2
    requested_ref=$3
    resolved_tag=$4
    message=$5
    binary_path=$6
    stable_binary_path=$7
    changed_state=$8

    if core_output_is_json; then
        python3 - "$core_type" "$action" "$requested_ref" "$resolved_tag" "$message" "$binary_path" "$stable_binary_path" "$changed_state" <<'PY'
import json, sys
core_type, action, requested_ref, resolved_tag, message, binary_path, stable_binary_path, changed_state = sys.argv[1:9]
payload = {
    'success': True,
    'core_type': core_type,
    'action': action,
    'requested_ref': requested_ref,
    'resolved_tag': resolved_tag,
    'message': message,
    'binary_path': binary_path,
    'stable_binary_path': stable_binary_path,
}
if changed_state in ('true', 'false'):
    payload['changed'] = changed_state == 'true'
print(json.dumps(payload, separators=(',', ':')))
PY
        return 0
    fi

    echo "$message"
    return 0
}

normalize_core_channel() {
    normalized_channel=$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')
    case "$normalized_channel" in
        "")
            printf '%s' ""
            ;;
        latest|stable)
            printf '%s' "latest"
            ;;
        *)
            echo "Error: unsupported core channel '${1}'. Use stable or latest."
            return 1
            ;;
    esac
}

normalize_digest_value() {
    digest=$1
    digest=$(printf '%s' "$digest" | tr -d '\r\n')
    case "$digest" in
        sha256:*) printf '%s' "${digest#sha256:}" ;;
        *) printf '%s' "$digest" ;;
    esac
}

verify_digest_value() {
    file_path=$1
    digest_value=$2
    asset_name=$3

    normalized=$(normalize_digest_value "$digest_value")
    if [ -z "$normalized" ]; then
        echo "Error: missing digest for ${asset_name}."
        return 1
    fi

    actual=$(hash_file_sha256 "$file_path")
    if [ "$actual" != "$normalized" ]; then
        echo "Error: checksum mismatch for ${asset_name}."
        echo "Expected: ${normalized}"
        echo "Actual:   ${actual}"
        return 1
    fi
    if ! core_output_is_json; then
        echo "Checksum verified: ${asset_name} (${actual})"
    fi
    return 0
}

parse_xray_dgst_sha256() {
    dgst_file=$1
    sed -n 's/^SHA2-256=[[:space:]]*//p' "$dgst_file" | head -n 1 | tr -d '\r'
}

resolve_release_manifest_checksum() {
    repo=$1
    version=$2
    asset_name=$3
    workdir=$4

    release_base="${XBOARD_RELEASE_BASE_URL%/}"
    checksum_file="$workdir/SHA256SUMS.txt"

    if [ "$version" = "" ] || [ "$version" = "latest" ]; then
        checksum_url="${release_base}/${repo}/releases/latest/download/SHA256SUMS.txt"
    else
        checksum_url="${release_base}/${repo}/releases/download/${version}/SHA256SUMS.txt"
    fi

    if ! download_file "$checksum_url" "$checksum_file" "checksum manifest"; then
        return 1
    fi

    checksum_value=$(lookup_expected_checksum "$asset_name" "$checksum_file" || true)
    if [ -z "$checksum_value" ]; then
        echo "Error: checksum entry not found for ${asset_name} in release manifest."
        return 1
    fi
    printf 'sha256:%s' "$checksum_value"
    return 0
}
resolve_core_asset_digest() {
    metadata_file=$1
    core_type=$2
    asset_name=$3
    workdir=$4

    asset_digest=$(json_get_asset_field "$metadata_file" "$asset_name" "digest")
    if [ -n "$asset_digest" ]; then
        printf '%s' "$asset_digest"
        return 0
    fi

    release_tag=$(json_get_release_tag "$metadata_file")
    if [ -z "$release_tag" ]; then
        echo "Error: failed to resolve release tag while looking up digest for ${asset_name}."
        return 1
    fi

    if [ "$core_type" = "xray" ]; then
        dgst_name="${asset_name}.dgst"
        dgst_url=$(json_get_asset_field "$metadata_file" "$dgst_name" "browser_download_url")
        if [ -n "$dgst_url" ]; then
            dgst_path="$workdir/$dgst_name"
            if ! download_file "$dgst_url" "$dgst_path" "$dgst_name"; then
                return 1
            fi
            dgst_digest=$(parse_xray_dgst_sha256 "$dgst_path")
            if [ -z "$dgst_digest" ]; then
                echo "Error: failed to parse SHA2-256 digest from ${dgst_name}."
                return 1
            fi
            printf 'sha256:%s' "$dgst_digest"
            return 0
        fi
    fi

    if manifest_digest=$(resolve_release_manifest_checksum "$CORE_RELEASE_REPO" "$release_tag" "$asset_name" "$workdir"); then
        printf '%s' "$manifest_digest"
        return 0
    fi

    if [ "$core_type" = "sing-box" ]; then
        return 0
    fi

    return 1
}

extract_archive() {
    archive_path=$1
    target_dir=$2

    case "$archive_path" in
        *.tar.gz|*.tgz)
            tar -xzf "$archive_path" -C "$target_dir"
            ;;
        *.zip)
            if ! require_command unzip; then
                return 1
            fi
            unzip -q "$archive_path" -d "$target_dir"
            ;;
        *)
            echo "Error: unsupported archive format: ${archive_path}"
            return 1
            ;;
    esac
}

copy_with_parent() {
    src_path=$1
    dst_path=$2

    dst_dir=$(dirname "$dst_path")
    if ! ensure_dir "$dst_dir"; then
        echo "Error: failed to create directory ${dst_dir}."
        return 1
    fi
    if ! install_file "$src_path" "$dst_path"; then
        return 1
    fi
    return 0
}

install_symlink_or_copy() {
    src_path=$1
    dst_path=$2

    dst_dir=$(dirname "$dst_path")
    if ! ensure_dir "$dst_dir"; then
        echo "Error: failed to create directory ${dst_dir}."
        return 1
    fi

    if ln -sfn "$src_path" "$dst_path" >/dev/null 2>&1; then
        return 0
    fi
    if run_privileged ln -sfn "$src_path" "$dst_path" >/dev/null 2>&1; then
        return 0
    fi
    if ! install_executable_file "$src_path" "$dst_path"; then
        return 1
    fi
    return 0
}

persist_agent_deploy_assets() {
    deploy_dir="${INSTALL_DIR}/deploy"
    if ! ensure_dir "$deploy_dir"; then
        echo "Error: failed to create deploy directory ${deploy_dir}."
        return 1
    fi

    script_source=""
    if [ -f "$0" ]; then
        script_source=$0
    elif [ -f "${SCRIPT_DIR}/agent.sh" ]; then
        script_source="${SCRIPT_DIR}/agent.sh"
    fi
    if [ -n "$script_source" ]; then
        if ! install_executable_file "$script_source" "${deploy_dir}/agent.sh"; then
            echo "Error: failed to persist agent installer script."
            return 1
        fi
    fi

    service_source=$(resolve_service_file "agent.service")
    if [ -n "$service_source" ]; then
        if ! copy_with_parent "$service_source" "${deploy_dir}/agent.service"; then
            echo "Error: failed to persist agent service file."
            return 1
        fi
    fi
    return 0
}

resolve_core_asset() {
    core_type=$1
    version=$2
    flavor=$3

    CORE_RELEASE_REPO=""
    CORE_ASSET_NAME=""
    CORE_BINARY_NAME=""
    CORE_STABLE_BINARY_PATH=""

    case "$core_type" in
        sing-box)
            case "$flavor" in
                ""|official)
                    CORE_RELEASE_REPO="$XBOARD_SINGBOX_RELEASE_REPO"
                    ;;
                with-v2ray-api)
                    if [ -z "$XBOARD_SINGBOX_V2RAY_RELEASE_REPO" ]; then
                        echo "Error: with-v2ray-api flavor is not configured on this host."
                        return 1
                    fi
                    CORE_RELEASE_REPO="$XBOARD_SINGBOX_V2RAY_RELEASE_REPO"
                    ;;
                *)
                    echo "Error: unsupported sing-box flavor '${flavor}'."
                    return 1
                    ;;
            esac
            normalized_version=$version
            case "$normalized_version" in
                v*) normalized_version=${normalized_version#v} ;;
            esac
            case "$flavor" in
                with-v2ray-api)
                    CORE_ASSET_NAME="sing-box-linux-${ARCH}"
                    ;;
                *)
                    CORE_ASSET_NAME="sing-box-${normalized_version}-linux-${ARCH}.tar.gz"
                    ;;
            esac
            CORE_BINARY_NAME="sing-box"
            CORE_STABLE_BINARY_PATH="$XBOARD_SINGBOX_BINARY_PATH"
            ;;
        xray)
            if [ -n "$flavor" ] && [ "$flavor" != "official" ]; then
                echo "Error: unsupported xray flavor '${flavor}'."
                return 1
            fi
            CORE_RELEASE_REPO="$XBOARD_XRAY_RELEASE_REPO"
            case "$ARCH" in
                amd64) arch_token="64" ;;
                arm64) arch_token="arm64-v8a" ;;
                *)
                    echo "Error: unsupported xray architecture '${ARCH}'."
                    return 1
                    ;;
            esac
            CORE_ASSET_NAME="Xray-linux-${arch_token}.zip"
            CORE_BINARY_NAME="xray"
            CORE_STABLE_BINARY_PATH="$XBOARD_XRAY_BINARY_PATH"
            ;;
        *)
            echo "Error: unsupported core type '${core_type}'."
            return 1
            ;;
    esac

    return 0
}

find_extracted_binary() {
    search_dir=$1
    binary_name=$2

    python3 - "$search_dir" "$binary_name" <<'PY'
import os, sys
root, name = sys.argv[1], sys.argv[2]
for current_root, _, files in os.walk(root):
    if name in files:
        print(os.path.join(current_root, name))
        break
PY
}

resolve_downloaded_core_binary() {
    asset_path=$1
    extract_dir=$2
    binary_name=$3

    case "$asset_path" in
        *.tar.gz|*.tgz|*.zip)
            mkdir -p "$extract_dir"
            if ! extract_archive "$asset_path" "$extract_dir"; then
                return 1
            fi
            find_extracted_binary "$extract_dir" "$binary_name"
            return 0
            ;;
        *)
            printf '%s' "$asset_path"
            return 0
            ;;
    esac
}


install_core_release() {
    core_type=$1
    action=$2
    version=$3
    channel=$4
    flavor=$5

    if [ -n "$version" ] && [ -n "$channel" ]; then
        echo "Error: --core-version and --core-channel cannot be used together."
        return 1
    fi

    if [ -n "$channel" ]; then
        channel=$(normalize_core_channel "$channel") || return 1
    fi
    if [ -z "$version" ] && [ -z "$channel" ]; then
        channel="latest"
    fi

    requested_ref=$version
    if [ -z "$requested_ref" ]; then
        requested_ref=$channel
    fi

    if ! ensure_download_dependencies; then
        echo "Error: release download dependency check failed for core install."
        return 1
    fi
    if ! require_command python3; then
        return 1
    fi
    if [ "$core_type" = "sing-box" ]; then
        if ! ensure_dependency "tar"; then
            echo "Error: release download dependency check failed for sing-box archive extraction."
            return 1
        fi
    fi

    workdir=$(mktemp -d 2>/dev/null || mktemp -d -t xboard-core-install)
    if [ -z "$workdir" ]; then
        echo "Error: failed to create temporary core install directory."
        return 1
    fi
    trap 'rm -rf "$workdir"' EXIT INT TERM

    metadata_file="$workdir/release.json"
    if ! resolve_core_asset "$core_type" "$requested_ref" "$flavor"; then
        return 1
    fi
    if ! fetch_release_metadata "$CORE_RELEASE_REPO" "$requested_ref" "$metadata_file"; then
        return 1
    fi
    resolved_tag=$(json_get_release_tag "$metadata_file")
    if [ -z "$resolved_tag" ]; then
        echo "Error: failed to resolve release tag for ${core_type}."
        return 1
    fi

    if ! resolve_core_asset "$core_type" "$resolved_tag" "$flavor"; then
        return 1
    fi

    download_url=$(json_get_asset_field "$metadata_file" "$CORE_ASSET_NAME" "browser_download_url")
    if ! asset_digest=$(resolve_core_asset_digest "$metadata_file" "$core_type" "$CORE_ASSET_NAME" "$workdir"); then
        return 1
    fi
    if ! core_output_is_json; then
        echo "Core release: type=${core_type} action=${action} repo=${CORE_RELEASE_REPO} requested=${requested_ref} resolved=${resolved_tag} asset=${CORE_ASSET_NAME}"
    fi
    if [ -z "$download_url" ]; then
        echo "Error: release asset '${CORE_ASSET_NAME}' not found in ${CORE_RELEASE_REPO}@${resolved_tag}."
        return 1
    fi

    archive_path="$workdir/$CORE_ASSET_NAME"
    if ! download_file "$download_url" "$archive_path" "$CORE_ASSET_NAME"; then
        return 1
    fi
    if [ -n "$asset_digest" ]; then
        if ! verify_digest_value "$archive_path" "$asset_digest" "$CORE_ASSET_NAME"; then
            return 1
        fi
    fi

    extract_dir="$workdir/extracted"
    binary_path=$(resolve_downloaded_core_binary "$archive_path" "$extract_dir" "$CORE_BINARY_NAME")
    if [ -z "$binary_path" ] || [ ! -f "$binary_path" ]; then
        echo "Error: failed to locate downloaded binary '${CORE_BINARY_NAME}'."
        return 1
    fi

    version_dir="${XBOARD_CORE_INSTALL_BASE_DIR}/${core_type}/${resolved_tag}"
    target_binary_path="${version_dir}/${CORE_BINARY_NAME}"
    if [ "$action" = "ensure" ] && [ -x "$target_binary_path" ]; then
        if ! install_symlink_or_copy "$target_binary_path" "$CORE_STABLE_BINARY_PATH"; then
            return 1
        fi
        emit_core_install_success "$core_type" "$action" "$requested_ref" "$resolved_tag" "Core ${core_type} already installed at ${resolved_tag}." "$target_binary_path" "$CORE_STABLE_BINARY_PATH" "false"
        return 0
    fi

    if ! ensure_dir "$version_dir"; then
        echo "Error: failed to create version directory ${version_dir}."
        return 1
    fi
    if ! install_executable_file "$binary_path" "$target_binary_path"; then
        echo "Error: failed to install core binary into ${target_binary_path}."
        return 1
    fi
    if ! install_symlink_or_copy "$target_binary_path" "$CORE_STABLE_BINARY_PATH"; then
        return 1
    fi

    emit_core_install_success "$core_type" "$action" "$requested_ref" "$resolved_tag" "Installed ${core_type} ${resolved_tag} to ${target_binary_path}." "$target_binary_path" "$CORE_STABLE_BINARY_PATH" "true"
    return 0
}

hash_file_sha256() {
    target_path=$1

    if command -v sha256sum >/dev/null 2>&1; then
        set -- $(sha256sum "$target_path")
        printf '%s' "$1"
        return 0
    fi

    if command -v shasum >/dev/null 2>&1; then
        set -- $(shasum -a 256 "$target_path")
        printf '%s' "$1"
        return 0
    fi

    if command -v openssl >/dev/null 2>&1; then
        if openssl_output=$(openssl dgst -sha256 "$target_path" 2>/dev/null); then
            printf '%s' "${openssl_output##*= }"
            return 0
        fi
    fi

    echo "Error: no SHA256 tool found (requires sha256sum, shasum, or openssl)."
    return 1
}

lookup_expected_checksum() {
    wanted_name=$1
    checksum_file=$2

    while IFS= read -r line || [ -n "$line" ]; do
        case "$line" in
            ''|'#'*)
                continue
                ;;
        esac

        set -- $line
        checksum=$1
        listed_name=${2#*}
        listed_name=$(printf '%s' "$listed_name" | tr -d '\r')

        case "$listed_name" in
            "${wanted_name}"|"deploy/${wanted_name}"|"dist/release/${wanted_name}"|"./${wanted_name}"|"*/${wanted_name}")
                printf '%s' "$checksum"
                return 0
                ;;
        esac
    done < "$checksum_file"

    return 1
}

verify_checksum() {
    file_name=$1
    file_path=$2
    checksum_file=$3

    expected_checksum=$(lookup_expected_checksum "$file_name" "$checksum_file" || true)
    if [ -z "$expected_checksum" ]; then
        echo "Error: checksum entry not found for ${file_name}."
        return 1
    fi

    actual_checksum=$(hash_file_sha256 "$file_path")
    if [ "$actual_checksum" != "$expected_checksum" ]; then
        echo "Error: checksum mismatch for ${file_name}."
        echo "Expected: ${expected_checksum}"
        echo "Actual:   ${actual_checksum}"
        return 1
    fi

    echo "Checksum verified: ${file_name} (${actual_checksum})"
    return 0
}

install_binary() {
    bin_name=$1
    _cmd_path=$2

    target_bin="$bin_name"
    if [ "$OS" = "windows" ]; then
        target_bin="${bin_name}.exe"
    fi

    if ! ensure_install_dir; then
        return 1
    fi

    if ! download_release_binary "$bin_name" "$INSTALL_DIR/$target_bin"; then
        echo "Error: failed to install ${bin_name} from GitHub release asset."
        return 1
    fi

    return 0
}

resolve_service_file() {
    service_name=$1

    if [ "$service_name" = "agent.service" ] && [ -n "${XBOARD_AGENT_SERVICE_FILE:-}" ] && [ -f "${XBOARD_AGENT_SERVICE_FILE}" ]; then
        echo "${XBOARD_AGENT_SERVICE_FILE}"
        return 0
    fi

    if [ "$service_name" = "xboard.service" ] && [ -n "${XBOARD_PANEL_SERVICE_FILE:-}" ] && [ -f "${XBOARD_PANEL_SERVICE_FILE}" ]; then
        echo "${XBOARD_PANEL_SERVICE_FILE}"
        return 0
    fi

    if [ -n "${XBOARD_SERVICE_FILE:-}" ] && [ -f "${XBOARD_SERVICE_FILE}" ]; then
        echo "${XBOARD_SERVICE_FILE}"
        return 0
    fi

    if [ -f "${SCRIPT_DIR}/${service_name}" ]; then
        echo "${SCRIPT_DIR}/${service_name}"
    elif [ -f "deploy/${service_name}" ]; then
        echo "deploy/${service_name}"
    elif [ -f "${service_name}" ]; then
        echo "${service_name}"
    fi

    return 0
}

require_command() {
    cmd_name=$1
    if ! command -v "$cmd_name" >/dev/null 2>&1; then
        echo "Error: required command '${cmd_name}' not found."
        return 1
    fi
    return 0
}

download_file() {
    source_url=$1
    target_path=$2
    label=$3

    if ! curl --fail --silent --show-error --location --retry 3 --retry-delay 1 --output "$target_path" "$source_url"; then
        echo "Error: failed to download ${label} from ${source_url}."
        return 1
    fi

    if [ ! -s "$target_path" ]; then
        echo "Error: downloaded ${label} is empty."
        return 1
    fi

    return 0
}

is_commit_ref() {
    ref_value=$1
    if printf '%s' "$ref_value" | grep -Eq '^[0-9a-fA-F]{7,40}$'; then
        return 0
    fi
    return 1
}

resolve_latest_tag() {
    latest_meta_file=$1
    api_url="${XBOARD_BOOTSTRAP_API_BASE_URL%/}/repos/${XBOARD_BOOTSTRAP_REPO}/releases/latest"

    if ! download_file "$api_url" "$latest_meta_file" "latest release metadata"; then
        return 1
    fi

    latest_tag=$(sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$latest_meta_file" | head -n 1)
    if [ -z "$latest_tag" ]; then
        echo "Error: failed to parse latest release tag from ${api_url}."
        return 1
    fi

    printf '%s' "$latest_tag"
    return 0
}

run_bootstrap_mode() {
    if ! require_command curl; then
        return 1
    fi

    WORKDIR=$(mktemp -d 2>/dev/null || mktemp -d -t xboard-agent-bootstrap)
    if [ -z "$WORKDIR" ]; then
        echo "Error: failed to create temporary working directory."
        return 1
    fi

    cleanup_bootstrap() {
        if [ "$XBOARD_BOOTSTRAP_KEEP_TEMP" = "1" ]; then
            echo "Bootstrap temp directory retained: ${WORKDIR}"
            return
        fi

        rm -rf "$WORKDIR"
    }
    trap cleanup_bootstrap EXIT INT TERM

    LATEST_META_FILE="$WORKDIR/latest-release.json"
    if [ "$XBOARD_BOOTSTRAP_REF" = "latest" ]; then
        RESOLVED_REF=$(resolve_latest_tag "$LATEST_META_FILE")
        DEFAULT_RELEASE_TAG="$RESOLVED_REF"
    elif is_commit_ref "$XBOARD_BOOTSTRAP_REF"; then
        RESOLVED_REF="$XBOARD_BOOTSTRAP_REF"
        DEFAULT_RELEASE_TAG=""
    else
        RESOLVED_REF="$XBOARD_BOOTSTRAP_REF"
        DEFAULT_RELEASE_TAG="$RESOLVED_REF"
    fi

    if [ -n "$USER_XBOARD_RELEASE_TAG" ]; then
        RELEASE_TAG_TO_USE="$USER_XBOARD_RELEASE_TAG"
    else
        RELEASE_TAG_TO_USE="$DEFAULT_RELEASE_TAG"
    fi

    if [ -z "$RELEASE_TAG_TO_USE" ]; then
        echo "Error: bootstrap ref '${RESOLVED_REF}' looks like a commit hash."
        echo "Please set XBOARD_RELEASE_TAG to a release tag to keep script and binary versions consistent."
        return 1
    fi

    RAW_BASE="${XBOARD_BOOTSTRAP_RAW_BASE_URL%/}"
    RELEASE_BASE="${XBOARD_BOOTSTRAP_RELEASE_BASE_URL%/}"

    if [ -n "$XBOARD_AGENT_SCRIPT_URL" ]; then
        AGENT_URL="$XBOARD_AGENT_SCRIPT_URL"
    else
        AGENT_URL="${RAW_BASE}/${XBOARD_BOOTSTRAP_REPO}/${RESOLVED_REF}/deploy/agent.sh"
    fi
    if [ -n "$XBOARD_AGENT_SERVICE_URL" ]; then
        SERVICE_URL="$XBOARD_AGENT_SERVICE_URL"
    else
        SERVICE_URL="${RAW_BASE}/${XBOARD_BOOTSTRAP_REPO}/${RESOLVED_REF}/deploy/agent.service"
    fi

    CHECKSUM_URL="${RELEASE_BASE}/${XBOARD_BOOTSTRAP_REPO}/releases/download/${RELEASE_TAG_TO_USE}/SHA256SUMS.txt"

    if ! download_file "$AGENT_URL" "$WORKDIR/agent.sh" "agent.sh"; then
        return 1
    fi

    if ! download_file "$SERVICE_URL" "$WORKDIR/agent.service" "agent.service"; then
        echo "Error: failed to download agent.service from ${SERVICE_URL}."
        return 1
    fi

    if ! download_file "$CHECKSUM_URL" "$WORKDIR/checksums.txt" "checksum manifest"; then
        echo "Error: checksum manifest download failed: ${CHECKSUM_URL}"
        return 1
    fi

    if ! verify_checksum "agent.sh" "$WORKDIR/agent.sh" "$WORKDIR/checksums.txt"; then
        return 1
    fi
    if ! verify_checksum "agent.service" "$WORKDIR/agent.service" "$WORKDIR/checksums.txt"; then
        echo "Error: checksum verification failed for agent.service."
        return 1
    fi

    if [ -n "$USER_XBOARD_RELEASE_REPO" ]; then
        XBOARD_RELEASE_REPO="$USER_XBOARD_RELEASE_REPO"
    else
        XBOARD_RELEASE_REPO="$XBOARD_BOOTSTRAP_REPO"
    fi

    if [ -n "$USER_XBOARD_RELEASE_BASE_URL" ]; then
        XBOARD_RELEASE_BASE_URL="$USER_XBOARD_RELEASE_BASE_URL"
    else
        XBOARD_RELEASE_BASE_URL="$XBOARD_BOOTSTRAP_RELEASE_BASE_URL"
    fi

    XBOARD_RELEASE_TAG="$RELEASE_TAG_TO_USE"
    export XBOARD_RELEASE_REPO XBOARD_RELEASE_BASE_URL XBOARD_RELEASE_TAG

    XBOARD_AGENT_SERVICE_FILE="$WORKDIR/agent.service"
    export XBOARD_AGENT_SERVICE_FILE

    chmod +x "$WORKDIR/agent.sh"

    echo "Running agent installer (ref=${RESOLVED_REF}, release_tag=${XBOARD_RELEASE_TAG})..."
    (
        cd "$WORKDIR"
        sh ./agent.sh "$@"
    )
}

LEGACY_HOST_TOKEN="${XBOARD_AGENT_HOST_TOKEN:-}"
LEGACY_HOST_TOKEN_SET=0
LEGACY_HOST_TOKEN_SOURCE=""
if [ -n "$LEGACY_HOST_TOKEN" ]; then
    LEGACY_HOST_TOKEN_SET=1
    LEGACY_HOST_TOKEN_SOURCE="XBOARD_AGENT_HOST_TOKEN"
fi
COMMUNICATION_KEY="${XBOARD_AGENT_COMMUNICATION_KEY:-}"
COMMUNICATION_KEY_SET=0
if [ -n "$COMMUNICATION_KEY" ]; then
    COMMUNICATION_KEY_SET=1
fi
GRPC_ADDRESS="${XBOARD_AGENT_GRPC_ADDRESS:-}"
GRPC_ADDRESS_SET=0
if [ -n "$GRPC_ADDRESS" ]; then
    GRPC_ADDRESS_SET=1
fi
GRPC_TLS_ENABLED="${XBOARD_AGENT_GRPC_TLS_ENABLED:-false}"
GRPC_TLS_ENABLED_SET=0
if [ "${XBOARD_AGENT_GRPC_TLS_ENABLED+x}" = "x" ]; then
    GRPC_TLS_ENABLED_SET=1
fi
TRAFFIC_TYPE="${XBOARD_AGENT_TRAFFIC_TYPE:-netio}"
TRAFFIC_TYPE_SET=0
if [ "${XBOARD_AGENT_TRAFFIC_TYPE+x}" = "x" ]; then
    TRAFFIC_TYPE_SET=1
fi
FORCE_CONFIG_OVERWRITE="${XBOARD_AGENT_CONFIG_OVERWRITE:-0}"
FORCE_CONFIG_OVERWRITE_SET=0
if [ "$FORCE_CONFIG_OVERWRITE" = "1" ]; then
    FORCE_CONFIG_OVERWRITE_SET=1
fi
WITH_CORE_TYPE="${XBOARD_AGENT_WITH_CORE:-}"
WITH_CORE_TYPE_SET=0
if [ -n "$WITH_CORE_TYPE" ]; then
    WITH_CORE_TYPE_SET=1
fi
CORE_ACTION="${XBOARD_AGENT_CORE_ACTION:-}"
CORE_ACTION_SET=0
if [ -n "$CORE_ACTION" ]; then
    CORE_ACTION_SET=1
fi
CORE_TYPE="${XBOARD_AGENT_CORE_TYPE:-}"
CORE_TYPE_SET=0
if [ -n "$CORE_TYPE" ]; then
    CORE_TYPE_SET=1
fi
CORE_VERSION="${XBOARD_AGENT_CORE_VERSION:-}"
CORE_VERSION_SET=0
if [ -n "$CORE_VERSION" ]; then
    CORE_VERSION_SET=1
fi
CORE_CHANNEL="${XBOARD_AGENT_CORE_CHANNEL:-}"
CORE_CHANNEL_SET=0
if [ -n "$CORE_CHANNEL" ]; then
    CORE_CHANNEL_SET=1
fi
CORE_FLAVOR="${XBOARD_AGENT_CORE_FLAVOR:-}"
CORE_FLAVOR_SET=0
if [ -n "$CORE_FLAVOR" ]; then
    CORE_FLAVOR_SET=1
fi
BOOTSTRAP_MODE=0
UNINSTALL_MODE=0
BOOTSTRAP_REF_SET=0
BOOTSTRAP_REPO_SET=0
SERVICE_URL_SET=0

print_usage() {
    cat <<'EOF'
Usage:
  Install agent:
    sh agent.sh -k <communication-key> -g <grpc-address> [options]

  Install agent + core:
    sh agent.sh -k <communication-key> -g <grpc-address> -c <sing-box|xray> [core options]

  Bootstrap remote install:
    sh agent.sh --bootstrap [--ref <latest|tag|commit>] [--repo <owner/repo>] [--service-url <url>] -- -k <communication-key> -g <grpc-address> [options]

  Core maintenance only:
    sh agent.sh --core-action <install|upgrade|ensure> --core-type <sing-box|xray> [core options]

  Uninstall:
    sh agent.sh --uninstall

Install options:
  -k, --communication-key <key>  Agent registration communication key (required)
  -g, --grpc-address <address>   Panel gRPC address, e.g. 10.0.0.2:9090 (required)
  -t, --grpc-tls-enabled <bool>  true/false, default false
      --traffic-type <type>      traffic.type, default netio
  -f, --force-config-overwrite   overwrite existing config.yml
  -c, --with-core <core_type>    install core during agent install (sing-box|xray)

Core maintenance options:
      --core-action <action>     core install action: install|upgrade|ensure
      --core-type <core_type>    target core type for core install/upgrade
      --core-version <version>   core release version
      --core-channel <channel>   core release channel: stable|latest
      --core-flavor <flavor>     core release flavor, e.g. official

Bootstrap options:
      --bootstrap                run bootstrap mode (download+verify+install)
      --ref <latest|tag|commit>  bootstrap source ref (default: latest)
      --repo <owner/repo>        bootstrap source repository
      --service-url <url>        bootstrap service URL override

Other options:
      --uninstall                remove agent artifacts managed by this script
                                 if systemd is unavailable, installer will try OpenRC
  -h, --help                     show this help message

Environment:
  XBOARD_AGENT_COMMUNICATION_KEY
  XBOARD_AGENT_GRPC_ADDRESS
  XBOARD_AGENT_GRPC_TLS_ENABLED
  XBOARD_AGENT_TRAFFIC_TYPE
  XBOARD_AGENT_CONFIG_OVERWRITE=1
  XBOARD_AGENT_WITH_CORE
  XBOARD_AGENT_CORE_ACTION
  XBOARD_AGENT_CORE_TYPE
  XBOARD_AGENT_CORE_VERSION
  XBOARD_AGENT_CORE_CHANNEL
  XBOARD_AGENT_CORE_FLAVOR

Bootstrap-related:
  XBOARD_BOOTSTRAP_REF, XBOARD_BOOTSTRAP_REPO
  XBOARD_BOOTSTRAP_RAW_BASE_URL, XBOARD_BOOTSTRAP_RELEASE_BASE_URL
  XBOARD_BOOTSTRAP_API_BASE_URL
  XBOARD_BOOTSTRAP_DOWNLOAD_STRICT (deprecated, bootstrap is strict-only)
  XBOARD_AGENT_SCRIPT_URL, XBOARD_AGENT_SERVICE_URL
EOF
}

normalize_bool() {
    case "$1" in
        1|true|TRUE|yes|YES)
            printf '%s' "true"
            ;;
        0|false|FALSE|no|NO)
            printf '%s' "false"
            ;;
        *)
            return 1
            ;;
    esac
}

fail_host_token_disabled() {
    echo "Error: host_token can only be written back by the Agent after first-boot registration; do not pass --host-token or XBOARD_AGENT_HOST_TOKEN."
    exit 1
}

fail_host_token_mixed() {
    echo "Error: communication_key and host_token cannot be used together."
    echo "host_token can only be written back by the Agent after first-boot registration."
    exit 1
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --host-token)
            if [ "$#" -lt 2 ]; then
                echo "Error: --host-token requires a value."
                exit 1
            fi
            LEGACY_HOST_TOKEN=$2
            LEGACY_HOST_TOKEN_SET=1
            LEGACY_HOST_TOKEN_SOURCE="--host-token"
            shift 2
            ;;
        -k|--communication-key)
            if [ "$#" -lt 2 ]; then
                echo "Error: --communication-key requires a value."
                exit 1
            fi
            COMMUNICATION_KEY=$2
            COMMUNICATION_KEY_SET=1
            shift 2
            ;;
        -g|--grpc-address)
            if [ "$#" -lt 2 ]; then
                echo "Error: --grpc-address requires a value."
                exit 1
            fi
            GRPC_ADDRESS=$2
            GRPC_ADDRESS_SET=1
            shift 2
            ;;
        -t|--grpc-tls-enabled)
            if [ "$#" -lt 2 ]; then
                echo "Error: --grpc-tls-enabled requires a value."
                exit 1
            fi
            GRPC_TLS_ENABLED=$2
            GRPC_TLS_ENABLED_SET=1
            shift 2
            ;;
        --traffic-type)
            if [ "$#" -lt 2 ]; then
                echo "Error: --traffic-type requires a value."
                exit 1
            fi
            TRAFFIC_TYPE=$2
            TRAFFIC_TYPE_SET=1
            shift 2
            ;;
        -f|--force-config-overwrite)
            FORCE_CONFIG_OVERWRITE=1
            FORCE_CONFIG_OVERWRITE_SET=1
            shift
            ;;
        -c|--with-core)
            if [ "$#" -lt 2 ]; then
                echo "Error: --with-core requires a value."
                exit 1
            fi
            WITH_CORE_TYPE=$2
            WITH_CORE_TYPE_SET=1
            shift 2
            ;;
        --core-action)
            if [ "$#" -lt 2 ]; then
                echo "Error: --core-action requires a value."
                exit 1
            fi
            CORE_ACTION=$2
            CORE_ACTION_SET=1
            shift 2
            ;;
        --core-type)
            if [ "$#" -lt 2 ]; then
                echo "Error: --core-type requires a value."
                exit 1
            fi
            CORE_TYPE=$2
            CORE_TYPE_SET=1
            shift 2
            ;;
        --core-version)
            if [ "$#" -lt 2 ]; then
                echo "Error: --core-version requires a value."
                exit 1
            fi
            CORE_VERSION=$2
            CORE_VERSION_SET=1
            shift 2
            ;;
        --core-channel)
            if [ "$#" -lt 2 ]; then
                echo "Error: --core-channel requires a value."
                exit 1
            fi
            CORE_CHANNEL=$2
            CORE_CHANNEL_SET=1
            shift 2
            ;;
        --core-flavor)
            if [ "$#" -lt 2 ]; then
                echo "Error: --core-flavor requires a value."
                exit 1
            fi
            CORE_FLAVOR=$2
            CORE_FLAVOR_SET=1
            shift 2
            ;;
        --bootstrap)
            BOOTSTRAP_MODE=1
            shift
            ;;
        --uninstall)
            UNINSTALL_MODE=1
            shift
            ;;
        --ref)
            if [ "$#" -lt 2 ]; then
                echo "Error: --ref requires a value."
                exit 1
            fi
            XBOARD_BOOTSTRAP_REF=$2
            BOOTSTRAP_REF_SET=1
            shift 2
            ;;
        --repo)
            if [ "$#" -lt 2 ]; then
                echo "Error: --repo requires a value."
                exit 1
            fi
            XBOARD_BOOTSTRAP_REPO=$2
            BOOTSTRAP_REPO_SET=1
            shift 2
            ;;
        --service-url)
            if [ "$#" -lt 2 ]; then
                echo "Error: --service-url requires a value."
                exit 1
            fi
            XBOARD_AGENT_SERVICE_URL=$2
            SERVICE_URL_SET=1
            shift 2
            ;;
        -h|--help)
            print_usage
            exit 0
            ;;
        --)
            shift
            break
            ;;
        *)
            echo "Error: unknown argument: $1"
            print_usage
            exit 1
            ;;
    esac
done

if [ "$BOOTSTRAP_MODE" = "1" ] && [ "$UNINSTALL_MODE" = "1" ]; then
    echo "Error: --bootstrap and --uninstall cannot be used together."
    exit 1
fi

if [ "$COMMUNICATION_KEY_SET" = "1" ] && [ "$LEGACY_HOST_TOKEN_SET" = "1" ]; then
    fail_host_token_mixed
fi

if [ "$LEGACY_HOST_TOKEN_SET" = "1" ] && {
    [ "$LEGACY_HOST_TOKEN_SOURCE" = "--host-token" ] ||
    [ "$BOOTSTRAP_MODE" = "1" ] ||
    { [ "$UNINSTALL_MODE" != "1" ] && [ -z "$CORE_ACTION" ]; }
}; then
    fail_host_token_disabled
fi

if [ -n "$WITH_CORE_TYPE" ] && [ -n "$CORE_ACTION" ]; then
    echo "Error: --with-core cannot be combined with --core-action."
    exit 1
fi

if [ -n "$CORE_VERSION" ] && [ -n "$CORE_CHANNEL" ]; then
    echo "Error: --core-version and --core-channel cannot be used together."
    exit 1
fi

if [ -n "$WITH_CORE_TYPE" ] && [ -n "$CORE_TYPE" ] && [ "$WITH_CORE_TYPE" != "$CORE_TYPE" ]; then
    echo "Error: --with-core and --core-type must reference the same core."
    exit 1
fi

if [ -n "$WITH_CORE_TYPE" ] && [ -z "$CORE_TYPE" ]; then
    CORE_TYPE=$WITH_CORE_TYPE
fi

if [ -z "$WITH_CORE_TYPE" ] && [ -z "$CORE_ACTION" ] && { [ -n "$CORE_TYPE" ] || [ -n "$CORE_VERSION" ] || [ -n "$CORE_CHANNEL" ] || [ -n "$CORE_FLAVOR" ]; }; then
    echo "Error: core options require --with-core or --core-action."
    exit 1
fi

if [ -n "$CORE_ACTION" ] && [ -z "$CORE_TYPE" ]; then
    echo "Error: --core-type is required when --core-action is set."
    exit 1
fi

if [ -n "$CORE_ACTION" ] && [ "$BOOTSTRAP_MODE" = "1" ]; then
    echo "Error: --core-action cannot be combined with --bootstrap."
    exit 1
fi

if [ -n "$CORE_ACTION" ] && { [ "$COMMUNICATION_KEY_SET" = "1" ] || [ "$GRPC_ADDRESS_SET" = "1" ] || [ "$GRPC_TLS_ENABLED_SET" = "1" ] || [ "$TRAFFIC_TYPE_SET" = "1" ] || [ "$FORCE_CONFIG_OVERWRITE_SET" = "1" ] || [ "$BOOTSTRAP_REF_SET" = "1" ] || [ "$BOOTSTRAP_REPO_SET" = "1" ] || [ "$SERVICE_URL_SET" = "1" ]; }; then
    echo "Error: --core-action cannot be combined with install/bootstrap config parameters."
    exit 1
fi

if [ "$UNINSTALL_MODE" = "1" ]; then
    if [ "$COMMUNICATION_KEY_SET" = "1" ] || [ "$LEGACY_HOST_TOKEN_SET" = "1" ] || [ "$GRPC_ADDRESS_SET" = "1" ] || [ "$GRPC_TLS_ENABLED_SET" = "1" ] || [ "$TRAFFIC_TYPE_SET" = "1" ] || [ "$FORCE_CONFIG_OVERWRITE_SET" = "1" ] || [ "$WITH_CORE_TYPE_SET" = "1" ] || [ "$CORE_ACTION_SET" = "1" ] || [ "$CORE_TYPE_SET" = "1" ] || [ "$CORE_VERSION_SET" = "1" ] || [ "$CORE_CHANNEL_SET" = "1" ] || [ "$CORE_FLAVOR_SET" = "1" ] || [ "$BOOTSTRAP_REF_SET" = "1" ] || [ "$BOOTSTRAP_REPO_SET" = "1" ] || [ "$SERVICE_URL_SET" = "1" ]; then
        echo "Error: --uninstall cannot be combined with install/bootstrap/core parameters."
        exit 1
    fi

    echo "=== Uninstalling Agent ==="

    has_service_manager=0

    if is_systemd_available; then
        has_service_manager=1
        run_privileged systemctl disable --now xboard-agent >/dev/null 2>&1 || true
    else
        echo "Systemd is not available on this host. Skipping systemctl operations for xboard-agent."
    fi

    if is_openrc_available; then
        has_service_manager=1
        run_privileged "$OPENRC_SERVICE_CMD" xboard-agent stop >/dev/null 2>&1 || true
        run_privileged "$OPENRC_UPDATE_CMD" del xboard-agent default >/dev/null 2>&1 || run_privileged "$OPENRC_UPDATE_CMD" del xboard-agent >/dev/null 2>&1 || true
    fi

    if [ "$has_service_manager" = "0" ]; then
        echo "No supported service manager detected. Removed files only."
    fi

    run_privileged rm -f /etc/systemd/system/xboard-agent.service || true
    run_privileged rm -f /etc/init.d/xboard-agent || true

    if is_systemd_available; then
        if ! run_privileged systemctl daemon-reload; then
            echo "Error: failed to run systemctl daemon-reload."
            exit 1
        fi
    fi

    run_privileged rm -f "$INSTALL_DIR/agent" || true
    run_privileged rm -f "$INSTALL_DIR/config.yml" || true
    run_privileged rm -f "$INSTALL_DIR/agent_config.yml" || true

    echo "Agent uninstall completed."
    exit 0
fi

if [ "$BOOTSTRAP_MODE" = "1" ]; then
    export XBOARD_AGENT_COMMUNICATION_KEY="$COMMUNICATION_KEY"
    export XBOARD_AGENT_GRPC_ADDRESS="$GRPC_ADDRESS"
    export XBOARD_AGENT_GRPC_TLS_ENABLED="$GRPC_TLS_ENABLED"
    export XBOARD_AGENT_TRAFFIC_TYPE="$TRAFFIC_TYPE"
    export XBOARD_AGENT_CONFIG_OVERWRITE="$FORCE_CONFIG_OVERWRITE"
    export XBOARD_AGENT_WITH_CORE="$WITH_CORE_TYPE"
    export XBOARD_AGENT_CORE_ACTION="$CORE_ACTION"
    export XBOARD_AGENT_CORE_TYPE="$CORE_TYPE"
    export XBOARD_AGENT_CORE_VERSION="$CORE_VERSION"
    export XBOARD_AGENT_CORE_CHANNEL="$CORE_CHANNEL"
    export XBOARD_AGENT_CORE_FLAVOR="$CORE_FLAVOR"
    unset XBOARD_AGENT_HOST_TOKEN
    set_stage "bootstrap"
    if ! run_bootstrap_mode "$@"; then
        fail_stage "bootstrap install failed"
    fi
    exit 0
fi

if [ -n "$CORE_ACTION" ]; then
    if [ "$UNINSTALL_MODE" = "1" ]; then
        echo "Error: --core-action cannot be combined with --uninstall."
        exit 1
    fi
    set_stage "diagnostics"
    print_agent_install_summary
    set_stage "core install"
    if ! install_core_release "$CORE_TYPE" "$CORE_ACTION" "$CORE_VERSION" "$CORE_CHANNEL" "$CORE_FLAVOR"; then
        fail_stage "core installation failed"
    fi
    exit 0
fi

echo "=== Installing Agent ==="

set_stage "validate agent config"
GRPC_TLS_ENABLED_NORMALIZED=$(normalize_bool "$GRPC_TLS_ENABLED" || true)
if [ -z "$GRPC_TLS_ENABLED_NORMALIZED" ]; then
    echo "Error: invalid grpc tls flag '${GRPC_TLS_ENABLED}'. Expected true/false."
    fail_stage "invalid grpc tls flag"
fi
GRPC_TLS_ENABLED=$GRPC_TLS_ENABLED_NORMALIZED

if ! validate_agent_install_inputs; then
    fail_stage "agent config validation failed"
fi

set_stage "diagnostics"
print_agent_install_summary
print_agent_connectivity_summary

set_stage "prepare install directory"
if ! ensure_install_dir; then
    fail_stage "cannot create install directory"
fi

set_stage "check download dependencies"
if ! ensure_download_dependencies; then
    echo "Error: release download dependency check failed for agent."
    fail_stage "dependency check failed"
fi

set_stage "install agent binary"
if ! install_binary "agent" "./cmd/agent/main.go"; then
    fail_stage "agent binary installation failed"
fi

set_stage "persist deploy assets"
if ! persist_agent_deploy_assets; then
    fail_stage "failed to persist deploy assets"
fi

if [ -n "$WITH_CORE_TYPE" ]; then
    set_stage "install bundled core"
    if ! install_core_release "$WITH_CORE_TYPE" "install" "$CORE_VERSION" "$CORE_CHANNEL" "$CORE_FLAVOR"; then
        fail_stage "bundled core installation failed"
    fi
fi

set_stage "write agent config"
CONFIG_PATH="$INSTALL_DIR/config.yml"
LEGACY_CONFIG_PATH="$INSTALL_DIR/agent_config.yml"
if [ ! -f "$CONFIG_PATH" ] && [ -f "$LEGACY_CONFIG_PATH" ]; then
    if run_privileged mv "$LEGACY_CONFIG_PATH" "$CONFIG_PATH"; then
        echo "Detected legacy agent_config.yml and migrated it to config.yml (${CONFIG_PATH})."
    else
        echo "Error: failed to migrate legacy agent_config.yml to config.yml."
        fail_stage "legacy config migration failed"
    fi
fi
if [ -f "$CONFIG_PATH" ] && [ "$FORCE_CONFIG_OVERWRITE" != "1" ]; then
    echo "config.yml already exists. Keep existing file (use --force-config-overwrite to overwrite)."
else
    if [ -z "$GRPC_ADDRESS" ]; then
        echo "Error: missing required config parameters."
        echo "grpc address is required to initialize config.yml."
        echo "Example:"
        echo "  sh ./deploy/agent.sh -k '<communication-key>' -g '127.0.0.1:9090'"
        fail_stage "grpc address missing"
    fi

    if [ -z "$COMMUNICATION_KEY" ]; then
        echo "Error: missing required authentication parameters."
        echo "communication_key is required to initialize config.yml."
        echo "host_token can only be written back by the Agent after first-boot registration."
        fail_stage "communication key missing"
    fi

    umask 077
    cat > "$CONFIG_PATH" <<EOF
panel:
  host_token: ""
  communication_key: "${COMMUNICATION_KEY}"

grpc:
  enabled: true
  address: "${GRPC_ADDRESS}"
  tls:
    enabled: ${GRPC_TLS_ENABLED}

core:
  install_script_path: "${INSTALL_DIR}/deploy/agent.sh"
  singbox_binary_path: "${INSTALL_DIR}/bin/sing-box"
  xray_binary_path: "${INSTALL_DIR}/bin/xray"

traffic:
  type: "${TRAFFIC_TYPE}"
EOF
    echo "Initialized config.yml at ${CONFIG_PATH}."
fi

set_stage "install service"
if [ "$SKIP_SYSTEMD" = "1" ]; then
    echo "Skipping xboard-agent.service installation (XBOARD_INSTALL_SKIP_SYSTEMD=1)."
elif is_systemd_available; then
    SERVICE_FILE=$(resolve_service_file "agent.service")
    if [ -z "$SERVICE_FILE" ]; then
        # Fall back to downloading from GitHub (covers pipe-mode installs where
        # the local template is not available).
        SERVICE_URL="${XBOARD_AGENT_SERVICE_URL:-}"
        if [ -z "$SERVICE_URL" ]; then
            SERVICE_URL="https://raw.githubusercontent.com/${XBOARD_RELEASE_REPO}/main/deploy/agent.service"
        fi
        DOWNLOADED_SERVICE=$(mktemp)
        if [ -n "$DOWNLOADED_SERVICE" ]; then
            if curl --fail --silent --show-error --location --retry 3 --retry-delay 1 --output "$DOWNLOADED_SERVICE" "$SERVICE_URL"; then
                SERVICE_FILE="$DOWNLOADED_SERVICE"
            else
                rm -f "$DOWNLOADED_SERVICE"
                echo "Warning: failed to download agent.service from ${SERVICE_URL}."
            fi
        fi
    fi
    if [ -n "$SERVICE_FILE" ]; then
        if ! render_install_service_file "$SERVICE_FILE" /etc/systemd/system/xboard-agent.service; then
            echo "Error: failed to install xboard-agent.service."
            fail_stage "systemd service installation failed"
        fi
        if ! run_privileged systemctl daemon-reload; then
            echo "Error: failed to run systemctl daemon-reload."
            fail_stage "systemd daemon reload failed"
        fi
        if ! run_privileged systemctl enable xboard-agent; then
            echo "Error: failed to enable xboard-agent service."
            fail_stage "systemd service enable failed"
        fi
        if ! run_privileged systemctl start xboard-agent; then
            echo "Error: failed to start xboard-agent service."
            fail_stage "systemd service start failed"
        fi
        echo "xboard-agent.service installed."
    else
        echo "Warning: agent.service not found (checked override/env/local paths)."
    fi
elif is_openrc_available; then
    if ! install_openrc_service "xboard-agent" "${INSTALL_DIR}/agent" "--config ${INSTALL_DIR}/config.yml"; then
        fail_stage "OpenRC service installation failed"
    fi
else
    echo "No supported service manager detected (systemd/openrc). Please manage agent process manually."
fi

set_stage "completed"
echo "Agent install completed."
