#!/bin/sh
set -eu

find_nss_wrapper() {
	for candidate in /usr/lib/libnss_wrapper.so /lib/libnss_wrapper.so /usr/lib64/libnss_wrapper.so; do
		if [ -r "$candidate" ]; then
			echo "$candidate"
			return 0
		fi
	done

	return 1
}

ensure_home_dir() {
	home_dir="${HOME:-}"
	if [ -n "$home_dir" ] && [ -d "$home_dir" ] && [ -w "$home_dir" ]; then
		return 0
	fi

	home_dir="/tmp/rbs-home-$(id -u)"
	mkdir -p "$home_dir"
	export HOME="$home_dir"
}

ensure_user_mapping() {
	uid="$(id -u)"
	gid="$(id -g)"

	if grep -Eq "^[^:]+:[^:]*:${uid}:${gid}:" /etc/passwd; then
		ensure_home_dir
		return 0
	fi

	nss_wrapper="$(find_nss_wrapper || true)"
	if [ -z "$nss_wrapper" ]; then
		echo "warning: libnss_wrapper is not available; SSH-based rsync may fail for uid ${uid}" >&2
		ensure_home_dir
		return 0
	fi

	ensure_home_dir

	passwd_file="/tmp/rbs-passwd-${uid}"
	group_file="/tmp/rbs-group-${gid}"
	username="${RBS_CONTAINER_USER:-rbs}"
	groupname="${RBS_CONTAINER_GROUP:-rbs}"

	cp /etc/passwd "$passwd_file"
	cp /etc/group "$group_file"

	if existing_group="$(awk -F: -v target_gid="$gid" '$3 == target_gid { print $1; exit }' "$group_file")" && [ -n "$existing_group" ]; then
		groupname="$existing_group"
	else
		echo "${groupname}:x:${gid}:" >> "$group_file"
	fi

	echo "${username}:x:${uid}:${gid}:RBS container user:${HOME}:/sbin/nologin" >> "$passwd_file"
	export NSS_WRAPPER_PASSWD="$passwd_file"
	export NSS_WRAPPER_GROUP="$group_file"
	if [ -n "${LD_PRELOAD:-}" ]; then
		export LD_PRELOAD="${nss_wrapper} ${LD_PRELOAD}"
	else
		export LD_PRELOAD="$nss_wrapper"
	fi
}

ensure_user_mapping

exec "$@"