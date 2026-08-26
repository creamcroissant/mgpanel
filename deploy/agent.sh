#!/bin/sh
set -e

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

INSTALL_DIR="${INSTALL_DIR:-/opt/mgpanel/agent}"
SKIP_SYSTEMD="${MGPANEL_INSTALL_SKIP_SYSTEMD:-0}"

USER_MGPANEL_RELEASE_REPO="${MGPANEL_RELEASE_REPO:-}"
USER_MGPANEL_RELEASE_TAG="${MGPANEL_RELEASE_TAG:-}"
USER_MGPANEL_RELEASE_BASE_URL="${MGPANEL_RELEASE_BASE_URL:-}"

DEFAULT_MGPANEL_RELEASE_REPO="${USER_MGPANEL_RELEASE_REPO:-creamcroissant/mgpanel}"
DEFAULT_MGPANEL_RELEASE_TAG="${USER_MGPANEL_RELEASE_TAG:-latest}"
DEFAULT_MGPANEL_RELEASE_BASE_URL="${USER_MGPANEL_RELEASE_BASE_URL:-https://github.com}"
MGPANEL_RELEASE_REPO="$DEFAULT_MGPANEL_RELEASE_REPO"
MGPANEL_RELEASE_TAG="$DEFAULT_MGPANEL_RELEASE_TAG"
MGPANEL_RELEASE_BASE_URL="$DEFAULT_MGPANEL_RELEASE_BASE_URL"
# Deprecated compatibility flag. Release download is strict-only now.
: "${MGPANEL_RELEASE_DOWNLOAD_STRICT:=1}"

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
MGPANEL_INSTALL_SKIP_CONNECT_CHECK="${MGPANEL_INSTALL_SKIP_CONNECT_CHECK:-0}"

set_stage() {
    CURRENT_STAGE=$1
    echo "Stage: ${CURRENT_STAGE}"
}

print_failure_context() {
    echo "Install failed at stage: ${CURRENT_STAGE:-unknown}."
    echo "Install dir: ${INSTALL_DIR}"
    echo "Release: repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH}"
    echo "Log hints: systemd -> journalctl -u mgpanel-agent -n 100 --no-pager"
    echo "Log hints: OpenRC -> rc-service mgpanel-agent status; check /var/log/messages or /var/log/syslog"
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

    echo "Install summary:"
    echo "  component: agent"
    echo "  install_dir: ${INSTALL_DIR}"
    echo "  os/arch: ${OS}/${ARCH}"
    echo "  distro: ${distro}"
    echo "  init: $(detect_init_label)"
    echo "  package_manager: ${PKG_MANAGER:-not detected}"
    echo "  release: repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} base=${MGPANEL_RELEASE_BASE_URL}"
    echo "  release_asset: ${agent_asset}"
    echo "  grpc_address: ${GRPC_ADDRESS:-not configured}"
    echo "  auth: shared first registration token; host token is written back after first boot"
    print_required_command_summary "curl" "ca-certificates" "unzip"
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

    if [ "$MGPANEL_INSTALL_SKIP_CONNECT_CHECK" = "1" ]; then
        echo "  grpc: skipped (MGPANEL_INSTALL_SKIP_CONNECT_CHECK=1)"
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

    eval "override_pkg=\${MGPANEL_PKG_${dep_key}_${manager_key}:-}"
    if [ -z "$override_pkg" ]; then
        eval "override_pkg=\${MGPANEL_PKG_${dep_key}:-}"
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
    install_dir_placeholder="__MGPANEL_INSTALL_DIR__"
    escaped_install_dir=$(printf '%s' "$INSTALL_DIR" | sed 's/[&#\\]/\\&/g')
    escaped_install_root=$(printf '%s' "$install_root" | sed 's/[&#\\]/\\&/g')
    if ! sed -e "s#/opt/mgpanel/agent#${install_dir_placeholder}#g" -e "s#/opt/mgpanel#${escaped_install_root}#g" -e "s#${install_dir_placeholder}#${escaped_install_dir}#g" "$source_path" > "$temp_service"; then
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
description="mgpanel ${service_name} service"
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
    base="${MGPANEL_RELEASE_BASE_URL%/}"

    if [ "$MGPANEL_RELEASE_TAG" = "latest" ]; then
        url="${base}/${MGPANEL_RELEASE_REPO}/releases/latest/download/${asset}"
        checksum_url="${base}/${MGPANEL_RELEASE_REPO}/releases/latest/download/SHA256SUMS.txt"
    else
        url="${base}/${MGPANEL_RELEASE_REPO}/releases/download/${MGPANEL_RELEASE_TAG}/${asset}"
        checksum_url="${base}/${MGPANEL_RELEASE_REPO}/releases/download/${MGPANEL_RELEASE_TAG}/SHA256SUMS.txt"
    fi

    echo "Release asset: ${asset}"
    echo "Release source: repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} url=${url}"

    if ! command -v curl >/dev/null 2>&1; then
        echo "Error: curl not found for release download of ${bin_name}."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        return 1
    fi

    if ! has_ca_certificates; then
        echo "Error: CA certificates not found for release download of ${bin_name}."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        return 1
    fi

    tmp_bin=$(mktemp)
    if [ -z "$tmp_bin" ]; then
        echo "Error: failed to create temporary file for ${asset}."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
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
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        rm -f "$tmp_bin" "$tmp_checksums"
        return 1
    fi

    if [ ! -s "$tmp_bin" ]; then
        echo "Error: downloaded ${asset} is empty."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        rm -f "$tmp_bin" "$tmp_checksums"
        return 1
    fi

    if ! curl --fail --silent --show-error --location --retry 3 --retry-delay 1 --output "$tmp_checksums" "$checksum_url"; then
        echo "Error: failed to download checksum manifest SHA256SUMS.txt."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH} checksum_url=${checksum_url}"
        rm -f "$tmp_bin" "$tmp_checksums"
        return 1
    fi

    if [ ! -s "$tmp_checksums" ]; then
        echo "Error: downloaded checksum manifest is empty."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} checksum_url=${checksum_url}"
        rm -f "$tmp_bin" "$tmp_checksums"
        return 1
    fi

    if ! verify_checksum "$asset" "$tmp_bin" "$tmp_checksums"; then
        echo "Error: checksum verification failed for release asset ${asset}."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} checksum_url=${checksum_url}"
        rm -f "$tmp_bin" "$tmp_checksums"
        return 1
    fi

    if ! install_executable_file "$tmp_bin" "$target_path"; then
        echo "Error: failed to install ${asset} into ${target_path}."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        rm -f "$tmp_bin" "$tmp_checksums"
        return 1
    fi

    rm -f "$tmp_bin" "$tmp_checksums"
    echo "Installed ${bin_name} from release asset: ${url}"
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

resolve_service_file() {
    service_name=$1

    if [ "$service_name" = "agent.service" ] && [ -n "${MGPANEL_AGENT_SERVICE_FILE:-}" ] && [ -f "${MGPANEL_AGENT_SERVICE_FILE}" ]; then
        echo "${MGPANEL_AGENT_SERVICE_FILE}"
        return 0
    fi

    if [ "$service_name" = "mgpanel.service" ] && [ -n "${MGPANEL_PANEL_SERVICE_FILE:-}" ] && [ -f "${MGPANEL_PANEL_SERVICE_FILE}" ]; then
        echo "${MGPANEL_PANEL_SERVICE_FILE}"
        return 0
    fi

    if [ -n "${MGPANEL_SERVICE_FILE:-}" ] && [ -f "${MGPANEL_SERVICE_FILE}" ]; then
        echo "${MGPANEL_SERVICE_FILE}"
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

LEGACY_HOST_TOKEN="${MGPANEL_AGENT_HOST_TOKEN:-}"
LEGACY_HOST_TOKEN_SET=0
LEGACY_HOST_TOKEN_SOURCE=""
if [ -n "$LEGACY_HOST_TOKEN" ]; then
    LEGACY_HOST_TOKEN_SET=1
    LEGACY_HOST_TOKEN_SOURCE="MGPANEL_AGENT_HOST_TOKEN"
fi
COMMUNICATION_KEY="${MGPANEL_AGENT_COMMUNICATION_KEY:-}"
COMMUNICATION_KEY_SET=0
if [ -n "$COMMUNICATION_KEY" ]; then
    COMMUNICATION_KEY_SET=1
fi
GRPC_ADDRESS="${MGPANEL_AGENT_GRPC_ADDRESS:-}"
GRPC_ADDRESS_SET=0
if [ -n "$GRPC_ADDRESS" ]; then
    GRPC_ADDRESS_SET=1
fi
GRPC_TLS_ENABLED="${MGPANEL_AGENT_GRPC_TLS_ENABLED:-false}"
GRPC_TLS_ENABLED_SET=0
if [ "${MGPANEL_AGENT_GRPC_TLS_ENABLED+x}" = "x" ]; then
    GRPC_TLS_ENABLED_SET=1
fi
TRAFFIC_TYPE="${MGPANEL_AGENT_TRAFFIC_TYPE:-netio}"
TRAFFIC_TYPE_SET=0
if [ "${MGPANEL_AGENT_TRAFFIC_TYPE+x}" = "x" ]; then
    TRAFFIC_TYPE_SET=1
fi
FORCE_CONFIG_OVERWRITE="${MGPANEL_AGENT_CONFIG_OVERWRITE:-0}"
FORCE_CONFIG_OVERWRITE_SET=0
if [ "$FORCE_CONFIG_OVERWRITE" = "1" ]; then
    FORCE_CONFIG_OVERWRITE_SET=1
fi

UNINSTALL_MODE=0

print_usage() {
    cat <<'EOF'
Usage:
  Install agent:
    sh agent.sh -k <communication-key> -g <grpc-address> [options]

  Uninstall:
    sh agent.sh --uninstall

Install options:
  -k, --communication-key <key>  Agent registration communication key (required)
  -g, --grpc-address <address>   Panel gRPC address, e.g. 10.0.0.2:9090 (required)
  -t, --grpc-tls-enabled <bool>  true/false, default false
      --traffic-type <type>      traffic.type, default netio
  -f, --force-config-overwrite   overwrite existing config.yml

Other options:
      --uninstall                remove agent artifacts managed by this script
  -h, --help                     show this help message

Environment:
  MGPANEL_AGENT_COMMUNICATION_KEY
  MGPANEL_AGENT_GRPC_ADDRESS
  MGPANEL_AGENT_GRPC_TLS_ENABLED
  MGPANEL_AGENT_TRAFFIC_TYPE
  MGPANEL_AGENT_CONFIG_OVERWRITE=1
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
    echo "Error: host_token can only be written back by the Agent after first-boot registration; do not pass --host-token or MGPANEL_AGENT_HOST_TOKEN."
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
        --uninstall)
            UNINSTALL_MODE=1
            shift
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

if [ "$COMMUNICATION_KEY_SET" = "1" ] && [ "$LEGACY_HOST_TOKEN_SET" = "1" ]; then
    fail_host_token_mixed
fi

if [ "$LEGACY_HOST_TOKEN_SET" = "1" ] && {
    [ "$LEGACY_HOST_TOKEN_SOURCE" = "--host-token" ] ||
    [ "$UNINSTALL_MODE" != "1" ]
}; then
    fail_host_token_disabled
fi

if [ "$UNINSTALL_MODE" = "1" ]; then
    if [ "$COMMUNICATION_KEY_SET" = "1" ] || [ "$LEGACY_HOST_TOKEN_SET" = "1" ] || [ "$GRPC_ADDRESS_SET" = "1" ] || [ "$GRPC_TLS_ENABLED_SET" = "1" ] || [ "$TRAFFIC_TYPE_SET" = "1" ] || [ "$FORCE_CONFIG_OVERWRITE_SET" = "1" ]; then
        echo "Error: --uninstall cannot be combined with install parameters."
        exit 1
    fi

    echo "=== Uninstalling Agent ==="

    has_service_manager=0

    if is_systemd_available; then
        has_service_manager=1
        run_privileged systemctl disable --now mgpanel-agent >/dev/null 2>&1 || true
    else
        echo "Systemd is not available on this host. Skipping systemctl operations for mgpanel-agent."
    fi

    if is_openrc_available; then
        has_service_manager=1
        run_privileged "$OPENRC_SERVICE_CMD" mgpanel-agent stop >/dev/null 2>&1 || true
        run_privileged "$OPENRC_UPDATE_CMD" del mgpanel-agent default >/dev/null 2>&1 || run_privileged "$OPENRC_UPDATE_CMD" del mgpanel-agent >/dev/null 2>&1 || true
    fi

    if [ "$has_service_manager" = "0" ]; then
        echo "No supported service manager detected. Removed files only."
    fi

    run_privileged rm -f /etc/systemd/system/mgpanel-agent.service || true
    run_privileged rm -f /etc/init.d/mgpanel-agent || true

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

traffic:
  type: "${TRAFFIC_TYPE}"
EOF
    echo "Initialized config.yml at ${CONFIG_PATH}."
fi

set_stage "install service"
if [ "$SKIP_SYSTEMD" = "1" ]; then
    echo "Skipping mgpanel-agent.service installation (MGPANEL_INSTALL_SKIP_SYSTEMD=1)."
elif is_systemd_available; then
    SERVICE_FILE=$(resolve_service_file "agent.service")
    if [ -z "$SERVICE_FILE" ]; then
        # Fall back to downloading from GitHub (covers pipe-mode installs where
        # the local template is not available).
        SERVICE_URL="${MGPANEL_AGENT_SERVICE_URL:-}"
        if [ -z "$SERVICE_URL" ]; then
            SERVICE_URL="https://raw.githubusercontent.com/${MGPANEL_RELEASE_REPO}/main/deploy/agent.service"
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
        if ! render_install_service_file "$SERVICE_FILE" /etc/systemd/system/mgpanel-agent.service; then
            echo "Error: failed to install mgpanel-agent.service."
            fail_stage "systemd service installation failed"
        fi
        if ! run_privileged systemctl daemon-reload; then
            echo "Error: failed to run systemctl daemon-reload."
            fail_stage "systemd daemon reload failed"
        fi
        if ! run_privileged systemctl enable mgpanel-agent; then
            echo "Error: failed to enable mgpanel-agent service."
            fail_stage "systemd service enable failed"
        fi
        if ! run_privileged systemctl start mgpanel-agent; then
            echo "Error: failed to start mgpanel-agent service."
            fail_stage "systemd service start failed"
        fi
        echo "mgpanel-agent.service installed."
    else
        # 内置兑底模板：pipe 安装无本地模板且 raw.githubusercontent 不可达时仍能完成装服务。
        echo "Falling back to embedded default unit template."
        EMBEDDED_SERVICE=$(mktemp)
        cat > "$EMBEDDED_SERVICE" <<'EOF'
[Unit]
Description=MGPanel Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=__MGPANEL_INSTALL_DIR__
ExecStart=__MGPANEL_INSTALL_DIR__/agent --config __MGPANEL_INSTALL_DIR__/config.yml
Restart=on-failure
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
        SERVICE_FILE="$EMBEDDED_SERVICE"
        if ! render_install_service_file "$SERVICE_FILE" /etc/systemd/system/mgpanel-agent.service; then
            echo "Error: failed to install mgpanel-agent.service (embedded)."
            rm -f "$EMBEDDED_SERVICE"
            fail_stage "systemd service installation failed"
        fi
        rm -f "$EMBEDDED_SERVICE"
        run_privileged systemctl daemon-reload
        run_privileged systemctl enable mgpanel-agent
        run_privileged systemctl start mgpanel-agent || fail_stage "systemd service start failed"
        echo "mgpanel-agent.service installed (embedded template)."
    fi
elif is_openrc_available; then
    if ! install_openrc_service "mgpanel-agent" "${INSTALL_DIR}/agent" "--config ${INSTALL_DIR}/config.yml"; then
        fail_stage "OpenRC service installation failed"
    fi
else
    echo "No supported service manager detected (systemd/openrc). Please manage agent process manually."
fi

set_stage "post-checks"

# B5: mesh 依赖预检 —— wireguard-tools 缺失时尝试自动安装（失败不阻断 agent 本体）。
if ! command -v wg >/dev/null 2>&1; then
    echo "Warning: 'wg' (wireguard-tools) not found; mesh networking will be unavailable."
    WG_INSTALLED=0
    if command -v apt-get >/dev/null 2>&1; then
        run_privileged apt-get install -y wireguard-tools >/dev/null 2>&1 && WG_INSTALLED=1
    elif command -v dnf >/dev/null 2>&1; then
        run_privileged dnf install -y wireguard-tools >/dev/null 2>&1 && WG_INSTALLED=1
    elif command -v yum >/dev/null 2>&1; then
        run_privileged yum install -y wireguard-tools >/dev/null 2>&1 && WG_INSTALLED=1
    elif command -v apk >/dev/null 2>&1; then
        run_privileged apk add wireguard-tools >/dev/null 2>&1 && WG_INSTALLED=1
    fi
    if [ "$WG_INSTALLED" = "1" ]; then
        echo "wireguard-tools installed automatically."
    else
        echo "Warning: automatic install failed; install manually: apt/yum/apk install wireguard-tools"
    fi
fi

# 结尾健康摘要：任一关键项缺失即整体失败退出，杜绝“半装”假成功。
HEALTH_OK=1
echo "---- install health summary ----"
if [ "$SKIP_SYSTEMD" != "1" ]; then
    if run_privileged systemctl is-active --quiet mgpanel-agent; then
        echo "[OK] mgpanel-agent service: active"
    else
        echo "[FAIL] mgpanel-agent service: not active (check journalctl -u mgpanel-agent)"
        HEALTH_OK=0
    fi
fi
if [ -x "${INSTALL_DIR}/agent" ]; then
    echo "[OK] agent binary: ${INSTALL_DIR}/agent"
else
    echo "[FAIL] agent binary missing: ${INSTALL_DIR}/agent"
    HEALTH_OK=0
fi
if [ -f "${CONFIG_PATH}" ]; then
    echo "[OK] agent config: ${CONFIG_PATH}"
else
    echo "[FAIL] agent config missing: ${CONFIG_PATH}"
    HEALTH_OK=0
fi
if command -v wg >/dev/null 2>&1; then
    echo "[OK] wireguard tools: available"
else
    echo "[WARN] wireguard tools: unavailable (mesh disabled until installed)"
fi
echo "--------------------------------"

set_stage "completed"
if [ "$HEALTH_OK" != "1" ]; then
    fail_stage "install finished with failures (see health summary above)"
fi
echo "Agent install completed."
