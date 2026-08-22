#!/bin/sh
set -e

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

DEFAULT_INSTALL_DIR="/opt/mgpanel/panel"
INSTALL_DIR="${INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
FRONTEND_RELEASE_ASSET="${MGPANEL_FRONTEND_RELEASE_ASSET:-frontend-dist.tar.gz}"
INSTALL_UI_RELEASE_ASSET="${MGPANEL_INSTALL_UI_RELEASE_ASSET:-install-ui.tar.gz}"
SKIP_SYSTEMD="${MGPANEL_INSTALL_SKIP_SYSTEMD:-0}"

MGPANEL_RELEASE_REPO="${MGPANEL_RELEASE_REPO:-creamcroissant/mgpanel}"
MGPANEL_RELEASE_TAG="${MGPANEL_RELEASE_TAG:-latest}"
MGPANEL_RELEASE_BASE_URL="${MGPANEL_RELEASE_BASE_URL:-https://github.com}"
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
MGPANEL_INSTALL_SKIP_PORT_CHECK="${MGPANEL_INSTALL_SKIP_PORT_CHECK:-0}"
MGPANEL_PANEL_HTTP_PORT="${MGPANEL_PANEL_HTTP_PORT:-8080}"
MGPANEL_PANEL_GRPC_PORT="${MGPANEL_PANEL_GRPC_PORT:-$MGPANEL_PANEL_HTTP_PORT}"

set_stage() {
    CURRENT_STAGE=$1
    echo "Stage: ${CURRENT_STAGE}"
}

print_failure_context() {
    echo "Install failed at stage: ${CURRENT_STAGE:-unknown}."
    echo "Install dir: ${INSTALL_DIR}"
    echo "Release: repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH}"
    echo "Log hints: systemd -> journalctl -u mgpanel -n 100 --no-pager"
    echo "Log hints: OpenRC -> rc-service mgpanel status; check /var/log/messages or /var/log/syslog"
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

print_panel_install_summary() {
    load_os_release
    detect_pkg_manager >/dev/null 2>&1 || true

    distro="${DISTRO_ID:-unknown}"
    if [ -n "$DISTRO_ID_LIKE" ]; then
        distro="${distro} (like ${DISTRO_ID_LIKE})"
    fi

    panel_asset="mgpanel-${OS}-${ARCH}"
    if [ "$OS" = "windows" ]; then
        panel_asset="mgpanel-${OS}-${ARCH}.exe"
    fi

    echo "Install summary:"
    echo "  component: panel"
    echo "  install_dir: ${INSTALL_DIR}"
    echo "  os/arch: ${OS}/${ARCH}"
    echo "  distro: ${distro}"
    echo "  init: $(detect_init_label)"
    echo "  package_manager: ${PKG_MANAGER:-not detected}"
    echo "  release: repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} base=${MGPANEL_RELEASE_BASE_URL}"
    echo "  release_assets: ${panel_asset}, ${FRONTEND_RELEASE_ASSET}, ${INSTALL_UI_RELEASE_ASSET}"
    print_required_command_summary curl ca-certificates tar
}

port_in_use() {
    port_value=$1

    if command -v ss >/dev/null 2>&1; then
        if ss -ltn 2>/dev/null | grep -E "[:.]${port_value}[[:space:]]" >/dev/null 2>&1; then
            return 0
        fi
        return 1
    fi

    if command -v netstat >/dev/null 2>&1; then
        if netstat -ltn 2>/dev/null | grep -E "[:.]${port_value}[[:space:]]" >/dev/null 2>&1; then
            return 0
        fi
        return 1
    fi

    if command -v lsof >/dev/null 2>&1; then
        if lsof -nP -iTCP:"${port_value}" -sTCP:LISTEN >/dev/null 2>&1; then
            return 0
        fi
        return 1
    fi

    return 2
}

check_port_available() {
    label=$1
    port_value=$2

    if [ "$MGPANEL_INSTALL_SKIP_PORT_CHECK" = "1" ]; then
        echo "  ${label}: skipped (MGPANEL_INSTALL_SKIP_PORT_CHECK=1)"
        return 0
    fi

    if port_in_use "$port_value"; then
        port_status=0
    else
        port_status=$?
    fi
    case "$port_status" in
        0)
            echo "  ${label}: warning, port ${port_value} appears to be in use"
            ;;
        1)
            echo "  ${label}: port ${port_value} appears available"
            ;;
        *)
            echo "  ${label}: skipped, install ss/netstat/lsof for local port diagnostics"
            ;;
    esac
}

print_panel_port_summary() {
    echo "Port preflight (warning only):"
    check_port_available "http" "$MGPANEL_PANEL_HTTP_PORT"
    check_port_available "grpc" "$MGPANEL_PANEL_GRPC_PORT"
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

    if ! ensure_dependency "tar"; then
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
    # Linux allows unlinking an in-use executable (the running process
    # keeps the old inode); a fresh file can then be written at the same path.
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

commit_prepared_dir() {
    cpd_src_dir=$1
    cpd_target_dir=$2
    cpd_backup_dir=$3

    cpd_parent_dir=$(dirname "$cpd_target_dir")
    cpd_had_backup=0

    if [ ! -d "$cpd_src_dir" ]; then
        echo "Error: prepared directory not found: ${cpd_src_dir}."
        return 1
    fi

    if [ -z "$cpd_backup_dir" ]; then
        echo "Error: backup directory path is required for ${cpd_target_dir}."
        return 1
    fi

    if ! ensure_dir "$cpd_parent_dir"; then
        return 1
    fi

    if [ -e "$cpd_target_dir" ]; then
        if ! run_privileged mv "$cpd_target_dir" "$cpd_backup_dir"; then
            echo "Error: failed to move existing ${cpd_target_dir} to backup."
            return 1
        fi
        cpd_had_backup=1
    fi

    if ! run_privileged mv "$cpd_src_dir" "$cpd_target_dir"; then
        echo "Error: failed to move prepared directory into ${cpd_target_dir}."
        if [ "$cpd_had_backup" = "1" ]; then
            run_privileged rm -rf "$cpd_target_dir" >/dev/null 2>&1 || true
            if ! run_privileged mv "$cpd_backup_dir" "$cpd_target_dir"; then
                echo "Error: failed to restore backup directory ${cpd_backup_dir}."
            fi
        fi
        return 1
    fi

    return 0
}

commit_prepared_file() {
    cpf_src_path=$1
    cpf_target_path=$2
    cpf_backup_path=$3
    cpf_mode_value=${4:-}

    cpf_parent_dir=$(dirname "$cpf_target_path")
    cpf_base_name=$(basename "$cpf_target_path")
    cpf_next_path="${cpf_parent_dir}/.${cpf_base_name}.new.$$"
    cpf_had_backup=0

    if [ ! -f "$cpf_src_path" ]; then
        echo "Error: prepared file not found: ${cpf_src_path}."
        return 1
    fi

    if [ -z "$cpf_backup_path" ]; then
        echo "Error: backup file path is required for ${cpf_target_path}."
        return 1
    fi

    if ! ensure_dir "$cpf_parent_dir"; then
        return 1
    fi

    rm -f "$cpf_next_path" 2>/dev/null || run_privileged rm -f "$cpf_next_path" 2>/dev/null || true
    if ! run_privileged cp "$cpf_src_path" "$cpf_next_path"; then
        echo "Error: failed to copy prepared file into ${cpf_next_path}."
        run_privileged rm -f "$cpf_next_path" >/dev/null 2>&1 || true
        return 1
    fi

    if [ -n "$cpf_mode_value" ]; then
        if ! chmod "$cpf_mode_value" "$cpf_next_path" >/dev/null 2>&1; then
            if ! run_privileged chmod "$cpf_mode_value" "$cpf_next_path"; then
                echo "Error: failed to set file mode on ${cpf_next_path}."
                run_privileged rm -f "$cpf_next_path" >/dev/null 2>&1 || true
                return 1
            fi
        fi
    fi

    if [ -e "$cpf_target_path" ]; then
        if ! run_privileged mv "$cpf_target_path" "$cpf_backup_path"; then
            echo "Error: failed to move existing ${cpf_target_path} to backup."
            run_privileged rm -f "$cpf_next_path" >/dev/null 2>&1 || true
            return 1
        fi
        cpf_had_backup=1
    fi

    if ! run_privileged mv "$cpf_next_path" "$cpf_target_path"; then
        echo "Error: failed to move prepared file into ${cpf_target_path}."
        if [ "$cpf_had_backup" = "1" ]; then
            run_privileged rm -f "$cpf_target_path" >/dev/null 2>&1 || true
            if ! run_privileged mv "$cpf_backup_path" "$cpf_target_path"; then
                echo "Error: failed to restore backup file ${cpf_backup_path}."
            fi
        fi
        run_privileged rm -f "$cpf_next_path" >/dev/null 2>&1 || true
        return 1
    fi

    return 0
}

restore_committed_dir() {
    rcd_target_dir=$1
    rcd_backup_dir=$2
    rcd_had_backup=$3
    rcd_label=$4

    if [ "$rcd_had_backup" = "1" ]; then
        run_privileged rm -rf "$rcd_target_dir" >/dev/null 2>&1 || true
        if ! run_privileged mv "$rcd_backup_dir" "$rcd_target_dir"; then
            echo "Error: failed to restore ${rcd_label} backup ${rcd_backup_dir}."
            return 1
        fi
        return 0
    fi

    run_privileged rm -rf "$rcd_target_dir" >/dev/null 2>&1 || true
    return 0
}

restore_committed_file() {
    rcf_target_path=$1
    rcf_backup_path=$2
    rcf_had_backup=$3
    rcf_label=$4

    if [ "$rcf_had_backup" = "1" ]; then
        run_privileged rm -f "$rcf_target_path" >/dev/null 2>&1 || true
        if ! run_privileged mv "$rcf_backup_path" "$rcf_target_path"; then
            echo "Error: failed to restore ${rcf_label} backup ${rcf_backup_path}."
            return 1
        fi
        return 0
    fi

    run_privileged rm -f "$rcf_target_path" >/dev/null 2>&1 || true
    return 0
}

rollback_panel_release_commit() {
    rpr_failed=0

    if [ "${panel_binary_committed:-0}" = "1" ]; then
        if ! restore_committed_file "$panel_binary_target" "$panel_binary_backup" "$panel_binary_had_existing" "panel binary"; then
            rpr_failed=1
        fi
        panel_binary_committed=0
    fi

    if [ "${panel_install_ui_committed:-0}" = "1" ]; then
        if ! restore_committed_dir "$panel_install_ui_target" "$panel_install_ui_backup" "$panel_install_ui_had_existing" "install UI assets"; then
            rpr_failed=1
        fi
        panel_install_ui_committed=0
    fi

    if [ "${panel_frontend_committed:-0}" = "1" ]; then
        if ! restore_committed_dir "$panel_frontend_target" "$panel_frontend_backup" "$panel_frontend_had_existing" "frontend assets"; then
            rpr_failed=1
        fi
        panel_frontend_committed=0
    fi

    if [ "$rpr_failed" = "1" ]; then
        echo "Error: one or more panel release rollback steps failed."
        return 1
    fi
    return 0
}

cleanup_panel_release_backups() {
    if [ "${panel_binary_had_existing:-0}" = "1" ] && [ -n "${panel_binary_backup:-}" ]; then
        run_privileged rm -f "$panel_binary_backup" >/dev/null 2>&1 || true
    fi

    if [ "${panel_install_ui_had_existing:-0}" = "1" ] && [ -n "${panel_install_ui_backup:-}" ]; then
        run_privileged rm -rf "$panel_install_ui_backup" >/dev/null 2>&1 || true
    fi

    if [ "${panel_frontend_had_existing:-0}" = "1" ] && [ -n "${panel_frontend_backup:-}" ]; then
        run_privileged rm -rf "$panel_frontend_backup" >/dev/null 2>&1 || true
    fi
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

copy_recursive() {
    src_path=$1
    dst_path=$2

    if cp -r "$src_path" "$dst_path" >/dev/null 2>&1; then
        return 0
    fi

    run_privileged cp -r "$src_path" "$dst_path"
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
    if ! sed -e "s#/opt/mgpanel/panel#${install_dir_placeholder}#g" -e "s#/opt/mgpanel#${escaped_install_root}#g" -e "s#${install_dir_placeholder}#${escaped_install_dir}#g" "$source_path" > "$temp_service"; then
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

install_release_asset() {
    asset_name=$1
    target_path=$2

    base="${MGPANEL_RELEASE_BASE_URL%/}"

    if [ "$MGPANEL_RELEASE_TAG" = "latest" ]; then
        url="${base}/${MGPANEL_RELEASE_REPO}/releases/latest/download/${asset_name}"
        checksum_url="${base}/${MGPANEL_RELEASE_REPO}/releases/latest/download/SHA256SUMS.txt"
    else
        url="${base}/${MGPANEL_RELEASE_REPO}/releases/download/${MGPANEL_RELEASE_TAG}/${asset_name}"
        checksum_url="${base}/${MGPANEL_RELEASE_REPO}/releases/download/${MGPANEL_RELEASE_TAG}/SHA256SUMS.txt"
    fi

    echo "Release asset: ${asset_name}"
    echo "Release source: repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} url=${url}"

    if ! command -v curl >/dev/null 2>&1; then
        echo "Error: curl not found for release download of ${asset_name}."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        return 1
    fi

    if ! has_ca_certificates; then
        echo "Error: CA certificates not found for release download of ${asset_name}."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        return 1
    fi

    download_tmp_asset=$(mktemp)
    if [ -z "$download_tmp_asset" ]; then
        echo "Error: failed to create temporary file for ${asset_name}."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        return 1
    fi

    download_tmp_checksums=$(mktemp)
    if [ -z "$download_tmp_checksums" ]; then
        echo "Error: failed to create temporary file for checksum manifest."
        rm -f "$download_tmp_asset"
        return 1
    fi

    if ! curl --fail --silent --show-error --location --retry 3 --retry-delay 1 --output "$download_tmp_asset" "$url"; then
        echo "Error: failed to download release asset ${asset_name}."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        rm -f "$download_tmp_asset" "$download_tmp_checksums"
        return 1
    fi

    if [ ! -s "$download_tmp_asset" ]; then
        echo "Error: downloaded ${asset_name} is empty."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        rm -f "$download_tmp_asset" "$download_tmp_checksums"
        return 1
    fi

    if ! curl --fail --silent --show-error --location --retry 3 --retry-delay 1 --output "$download_tmp_checksums" "$checksum_url"; then
        echo "Error: failed to download checksum manifest SHA256SUMS.txt."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH} checksum_url=${checksum_url}"
        rm -f "$download_tmp_asset" "$download_tmp_checksums"
        return 1
    fi

    if [ ! -s "$download_tmp_checksums" ]; then
        echo "Error: downloaded checksum manifest is empty."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} checksum_url=${checksum_url}"
        rm -f "$download_tmp_asset" "$download_tmp_checksums"
        return 1
    fi

    if ! verify_checksum "$asset_name" "$download_tmp_asset" "$download_tmp_checksums"; then
        echo "Error: checksum verification failed for release asset ${asset_name}."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} checksum_url=${checksum_url}"
        rm -f "$download_tmp_asset" "$download_tmp_checksums"
        return 1
    fi

    if ! install_file "$download_tmp_asset" "$target_path"; then
        echo "Error: failed to install ${asset_name} into ${target_path}."
        echo "repo=${MGPANEL_RELEASE_REPO} tag=${MGPANEL_RELEASE_TAG} os=${OS} arch=${ARCH} url=${url}"
        rm -f "$download_tmp_asset" "$download_tmp_checksums"
        return 1
    fi

    rm -f "$download_tmp_asset" "$download_tmp_checksums"
    echo "Installed release asset: ${url}"
    return 0
}

install_release_archive_dir() {
    asset_name=$1
    extract_parent=$2
    extracted_dir_name=$3
    target_dir=$4

    archive_tmp_asset=$(mktemp)
    if [ -z "$archive_tmp_asset" ]; then
        echo "Error: failed to create temporary file for ${asset_name}."
        return 1
    fi

    archive_tmp_extract=$(mktemp -d)
    if [ -z "$archive_tmp_extract" ]; then
        echo "Error: failed to create temporary directory for ${asset_name}."
        rm -f "$archive_tmp_asset"
        return 1
    fi

    if ! install_release_asset "$asset_name" "$archive_tmp_asset"; then
        rm -f "$archive_tmp_asset"
        rm -rf "$archive_tmp_extract"
        return 1
    fi

    if ! tar -C "$archive_tmp_extract" -xzf "$archive_tmp_asset"; then
        echo "Error: failed to extract ${asset_name}."
        rm -f "$archive_tmp_asset"
        rm -rf "$archive_tmp_extract"
        return 1
    fi

    extracted_path="${archive_tmp_extract}/${extracted_dir_name}"
    if [ ! -d "$extracted_path" ]; then
        echo "Error: extracted directory ${extracted_dir_name} not found in ${asset_name}."
        rm -f "$archive_tmp_asset"
        rm -rf "$archive_tmp_extract"
        return 1
    fi

    if ! ensure_dir "$extract_parent"; then
        echo "Error: failed to create install directory ${extract_parent}."
        rm -f "$archive_tmp_asset"
        rm -rf "$archive_tmp_extract"
        return 1
    fi

    if ! run_privileged rm -rf "$target_dir"; then
        echo "Error: failed to clear target directory ${target_dir}."
        rm -f "$archive_tmp_asset"
        rm -rf "$archive_tmp_extract"
        return 1
    fi
    if ! run_privileged mv "$extracted_path" "$target_dir"; then
        echo "Error: failed to install extracted directory ${target_dir}."
        rm -f "$archive_tmp_asset"
        rm -rf "$archive_tmp_extract"
        return 1
    fi

    rm -f "$archive_tmp_asset"
    rm -rf "$archive_tmp_extract"
    echo "Installed ${asset_name} into ${target_dir}"
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

    asset_name="${target_bin}-${OS}-${ARCH}"
    if [ "$OS" = "windows" ]; then
        asset_name="${bin_name}-${OS}-${ARCH}.exe"
    else
        asset_name="${bin_name}-${OS}-${ARCH}"
    fi

    if ! install_release_asset "$asset_name" "$INSTALL_DIR/$target_bin"; then
        echo "Error: failed to install ${bin_name} from GitHub release asset."
        return 1
    fi

    if ! set_file_mode +x "$INSTALL_DIR/$target_bin"; then
        echo "Error: failed to set executable permission on $INSTALL_DIR/$target_bin."
        return 1
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

write_default_service_template() {
    service_name=$1
    temp_service=$(mktemp)
    if [ -z "$temp_service" ]; then
        echo "Error: failed to create temporary service template."
        return 1
    fi

    case "$service_name" in
        mgpanel.service)
            cat > "$temp_service" <<'EOF'
[Unit]
Description=MGPanel Panel Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/mgpanel/panel
EnvironmentFile=-/etc/default/mgpanel
ExecStart=/opt/mgpanel/panel/mgpanel serve --config /opt/mgpanel/panel/config.yml
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
            ;;
        *)
            rm -f "$temp_service"
            return 1
            ;;
    esac

    echo "$temp_service"
    return 0
}

print_usage() {
    cat <<'EOF'
Usage: sh panel.sh [options]

Options:
  --uninstall    remove panel artifacts managed by this script
  -h, --help     show this help message
EOF
}

run_uninstall_mode() {
    echo "=== Uninstalling Panel ==="

    has_service_manager=0

    if is_systemd_available; then
        has_service_manager=1
        run_privileged systemctl disable --now mgpanel >/dev/null 2>&1 || true
    else
        echo "Systemd is not available on this host. Skipping systemctl operations for mgpanel."
    fi

    if is_openrc_available; then
        has_service_manager=1
        run_privileged "$OPENRC_SERVICE_CMD" mgpanel stop >/dev/null 2>&1 || true
        run_privileged "$OPENRC_UPDATE_CMD" del mgpanel default >/dev/null 2>&1 || run_privileged "$OPENRC_UPDATE_CMD" del mgpanel >/dev/null 2>&1 || true
    fi

    if [ "$has_service_manager" = "0" ]; then
        echo "No supported service manager detected. Removed files only."
    fi

    run_privileged rm -f /etc/systemd/system/mgpanel.service || true
    run_privileged rm -f /etc/init.d/mgpanel || true

    if is_systemd_available; then
        if ! run_privileged systemctl daemon-reload; then
            echo "Error: failed to run systemctl daemon-reload."
            return 1
        fi
    fi

    run_privileged rm -f "$INSTALL_DIR/mgpanel" || true
    run_privileged rm -rf "$INSTALL_DIR/web/user-vite/dist" || true
    run_privileged rm -rf "$INSTALL_DIR/web/install" || true
    run_privileged rm -f "$INSTALL_DIR/config.yml" || true
    run_privileged rm -f "$INSTALL_DIR/.env" || true

    echo "Panel uninstall completed."
    return 0
}

UNINSTALL_MODE=0

while [ "$#" -gt 0 ]; do
    case "$1" in
        --uninstall)
            UNINSTALL_MODE=1
            shift
            ;;
        -h|--help)
            print_usage
            exit 0
            ;;
        *)
            echo "Error: unknown argument: $1"
            print_usage
            exit 1
            ;;
    esac
done

if [ "$UNINSTALL_MODE" = "1" ]; then
    if ! run_uninstall_mode; then
        exit 1
    fi
    exit 0
fi

echo "=== Installing Panel ==="

set_stage "diagnostics"
print_panel_install_summary
print_panel_port_summary

set_stage "prepare install directory"
if ! ensure_install_dir; then
    fail_stage "cannot create install directory"
fi

set_stage "check download dependencies"
if ! ensure_download_dependencies; then
    echo "Error: release download dependency check failed for mgpanel."
    fail_stage "dependency check failed"
fi

PREPARED_RELEASE_DIR=$(mktemp -d)
if [ -z "$PREPARED_RELEASE_DIR" ]; then
    fail_stage "cannot create temporary release staging directory"
fi

cleanup_prepared_release_dir() {
    if [ -n "${PREPARED_RELEASE_DIR:-}" ]; then
        rm -rf "$PREPARED_RELEASE_DIR"
    fi
}

panel_install_cleanup_on_exit() {
    cleanup_status=$?
    if [ "$cleanup_status" -ne 0 ]; then
        rollback_panel_release_commit || true
    fi
    cleanup_prepared_release_dir
}

panel_install_handle_signal() {
    trap - 2 15
    exit 130
}

trap panel_install_cleanup_on_exit 0
trap panel_install_handle_signal 2 15

set_stage "prepare release assets"
prepared_binary_name="mgpanel"
if [ "$OS" = "windows" ]; then
    prepared_binary_name="mgpanel.exe"
    prepared_binary_asset="mgpanel-${OS}-${ARCH}.exe"
else
    prepared_binary_asset="mgpanel-${OS}-${ARCH}"
fi
prepared_binary_path="$PREPARED_RELEASE_DIR/$prepared_binary_name"

# Download all release assets in parallel (binary, frontend, install-ui)
(
    install_release_asset "$prepared_binary_asset" "$prepared_binary_path" || exit 1
    set_file_mode +x "$prepared_binary_path" || exit 1
) & BINARY_PID=$!

(
    install_release_archive_dir "$FRONTEND_RELEASE_ASSET" "$PREPARED_RELEASE_DIR/web/user-vite" "dist" "$PREPARED_RELEASE_DIR/web/user-vite/dist" || exit 1
) & FRONTEND_PID=$!

(
    install_release_archive_dir "$INSTALL_UI_RELEASE_ASSET" "$PREPARED_RELEASE_DIR/web" "install" "$PREPARED_RELEASE_DIR/web/install" || exit 1
) & INSTALL_PID=$!

# Wait for all downloads to complete and check results
if ! wait $BINARY_PID; then
    cleanup_prepared_release_dir
    fail_stage "panel binary download failed"
fi
if ! wait $FRONTEND_PID; then
    cleanup_prepared_release_dir
    fail_stage "frontend asset preparation failed"
fi
if ! wait $INSTALL_PID; then
    cleanup_prepared_release_dir
    fail_stage "install UI asset preparation failed"
fi

if [ ! -f "$PREPARED_RELEASE_DIR/web/user-vite/dist/index.html" ]; then
    echo "Error: required index.html not found in ${FRONTEND_RELEASE_ASSET}."
    cleanup_prepared_release_dir
    fail_stage "frontend asset validation failed"
fi
if [ ! -f "$PREPARED_RELEASE_DIR/web/install/index.html" ]; then
    echo "Error: required index.html not found in ${INSTALL_UI_RELEASE_ASSET}."
    cleanup_prepared_release_dir
    fail_stage "install UI asset validation failed"
fi

panel_frontend_target="$INSTALL_DIR/web/user-vite/dist"
panel_frontend_parent=$(dirname "$panel_frontend_target")
panel_frontend_base=$(basename "$panel_frontend_target")
panel_frontend_backup="${panel_frontend_parent}/.${panel_frontend_base}.bak.$$"
panel_frontend_had_existing=0
if [ -e "$panel_frontend_target" ]; then
    panel_frontend_had_existing=1
fi
panel_frontend_committed=0

panel_install_ui_target="$INSTALL_DIR/web/install"
panel_install_ui_parent=$(dirname "$panel_install_ui_target")
panel_install_ui_base=$(basename "$panel_install_ui_target")
panel_install_ui_backup="${panel_install_ui_parent}/.${panel_install_ui_base}.bak.$$"
panel_install_ui_had_existing=0
if [ -e "$panel_install_ui_target" ]; then
    panel_install_ui_had_existing=1
fi
panel_install_ui_committed=0

panel_binary_target="$INSTALL_DIR/$prepared_binary_name"
panel_binary_parent=$(dirname "$panel_binary_target")
panel_binary_base=$(basename "$panel_binary_target")
panel_binary_backup="${panel_binary_parent}/.${panel_binary_base}.bak.$$"
panel_binary_had_existing=0
if [ -e "$panel_binary_target" ]; then
    panel_binary_had_existing=1
fi
panel_binary_committed=0

set_stage "install frontend assets"
if ! commit_prepared_dir "$PREPARED_RELEASE_DIR/web/user-vite/dist" "$panel_frontend_target" "$panel_frontend_backup"; then
    cleanup_prepared_release_dir
    fail_stage "frontend asset installation failed"
fi
panel_frontend_committed=1
echo "Installed ${FRONTEND_RELEASE_ASSET} into $panel_frontend_target"

set_stage "install setup UI assets"
if ! commit_prepared_dir "$PREPARED_RELEASE_DIR/web/install" "$panel_install_ui_target" "$panel_install_ui_backup"; then
    rollback_panel_release_commit || true
    cleanup_prepared_release_dir
    fail_stage "install UI asset installation failed"
fi
panel_install_ui_committed=1
echo "Installed ${INSTALL_UI_RELEASE_ASSET} into $panel_install_ui_target"

set_stage "install panel binary"
if ! commit_prepared_file "$prepared_binary_path" "$panel_binary_target" "$panel_binary_backup" +x; then
    rollback_panel_release_commit || true
    cleanup_prepared_release_dir
    fail_stage "panel binary installation failed"
fi
panel_binary_committed=1
cleanup_panel_release_backups
cleanup_prepared_release_dir
trap - 0 2 15

set_stage "write panel config"
if [ ! -f "$INSTALL_DIR/config.yml" ] && [ ! -f "$INSTALL_DIR/.env" ]; then
    if [ -f "config.example.yml" ]; then
        if ! install_file "config.example.yml" "$INSTALL_DIR/config.yml"; then
            echo "Error: failed to create config.yml."
            fail_stage "panel config creation failed"
        fi
        echo "Created config.yml from local config.example.yml."
    elif [ -f ".env.example" ]; then
        if ! install_file ".env.example" "$INSTALL_DIR/.env"; then
            echo "Error: failed to create .env."
            fail_stage "panel env creation failed"
        fi
        echo "Created .env."
    else
        # Fallback: download config.example.yml from GitHub release
        echo "config.example.yml not found locally, downloading from GitHub..."
        EXAMPLE_URL="https://raw.githubusercontent.com/${MGPANEL_RELEASE_REPO}/main/config.example.yml"
        TMP_CONFIG=$(mktemp)
        if [ -n "$TMP_CONFIG" ] && curl --fail --silent --show-error --location --retry 3 --retry-delay 1 --output "$TMP_CONFIG" "$EXAMPLE_URL"; then
            if install_file "$TMP_CONFIG" "$INSTALL_DIR/config.yml"; then
                echo "Created config.yml from release config.example.yml."
            else
                echo "Error: failed to install downloaded config.yml."
                rm -f "$TMP_CONFIG"
                fail_stage "panel config creation failed"
            fi
            rm -f "$TMP_CONFIG"
        else
            rm -f "$TMP_CONFIG"
            # Last resort: generate minimal inline config
            echo "Warning: could not download config.example.yml, generating minimal config."
            CONFIG_TEMP=$(mktemp)
            if [ -z "$CONFIG_TEMP" ]; then
                fail_stage "failed to create temporary config file"
            fi
            cat > "$CONFIG_TEMP" <<EOF
# MGPanel Configuration

http:
  addr: "0.0.0.0:${MGPANEL_PANEL_HTTP_PORT:-8080}"
  shutdown_timeout: "15s"

log:
  level: "info"
  format: "json"
  add_source: false
  environment: "production"
  log_dir: "logs"
  max_days: 7

database:
  driver: "sqlite"
  path: "data/mgpanel.db"

auth:
  signing_key: "change-me"
EOF
            if install_file "$CONFIG_TEMP" "$INSTALL_DIR/config.yml"; then
                echo "Created minimal config.yml at ${INSTALL_DIR}."
            else
                echo "Error: failed to create config.yml."
                rm -f "$CONFIG_TEMP"
                fail_stage "panel config creation failed"
            fi
            rm -f "$CONFIG_TEMP"
        fi
    fi
fi

set_stage "install service"
if [ "$SKIP_SYSTEMD" = "1" ]; then
    echo "Skipping mgpanel.service installation (MGPANEL_INSTALL_SKIP_SYSTEMD=1)."
elif is_systemd_available; then
    SERVICE_FILE=$(resolve_service_file "mgpanel.service")
    TEMPLATE_SOURCE="$SERVICE_FILE"
    TEMP_TEMPLATE=""
    if [ -z "$TEMPLATE_SOURCE" ]; then
        if TEMP_TEMPLATE=$(write_default_service_template "mgpanel.service"); then
            TEMPLATE_SOURCE="$TEMP_TEMPLATE"
            echo "mgpanel.service template not found; using embedded default template."
        fi
    fi
    if [ -n "$TEMPLATE_SOURCE" ]; then
        if ! render_install_service_file "$TEMPLATE_SOURCE" /etc/systemd/system/mgpanel.service; then
            if [ -n "$TEMP_TEMPLATE" ]; then
                rm -f "$TEMP_TEMPLATE"
            fi
            echo "Error: failed to install mgpanel.service."
            fail_stage "systemd service installation failed"
        fi
        if [ -n "$TEMP_TEMPLATE" ]; then
            rm -f "$TEMP_TEMPLATE"
        fi
        if ! run_privileged systemctl daemon-reload; then
            echo "Error: failed to run systemctl daemon-reload."
            fail_stage "systemd daemon reload failed"
        fi
        if ! run_privileged systemctl enable mgpanel; then
            echo "Error: failed to enable mgpanel service."
            fail_stage "systemd service enable failed"
        fi
        if ! run_privileged systemctl start mgpanel; then
            echo "Error: failed to start mgpanel service."
            fail_stage "systemd service start failed"
        fi
        echo "mgpanel.service installed."
    else
        echo "Warning: deploy/mgpanel.service not found and embedded template generation failed."
    fi
elif is_openrc_available; then
    if ! install_openrc_service "mgpanel" "${INSTALL_DIR}/mgpanel" "serve --config ${INSTALL_DIR}/config.yml"; then
        fail_stage "OpenRC service installation failed"
    fi
else
    echo "No supported service manager detected (systemd/openrc). Please manage panel process manually."
fi

set_stage "completed"
echo "Panel install completed."

