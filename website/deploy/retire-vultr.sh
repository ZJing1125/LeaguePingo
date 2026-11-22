#!/usr/bin/env bash
# Remove files created by the retired LeaguePingo Ubuntu mirror installer.
set -euo pipefail
case "${1:-}" in
  '') apply=false ;;
  --apply) apply=true ;;
  *) echo 'Usage: sudo bash retire-vultr.sh [--apply]' >&2; exit 2 ;;
esac
[[ $EUID == 0 ]] || { echo 'Run as root to inspect the deployment.' >&2; exit 1; }
root=/srv/leaguepingo
library=/usr/local/lib/leaguepingo
conf=/etc/nginx/conf.d/leaguepingo-download.conf
units=/etc/systemd/system
service=leaguepingo-mirror.service
timer=leaguepingo-mirror.timer
user=leaguepingo-mirror
marker=.leaguepingo-managed
regular() { [[ -f "$1" && ! -L "$1" ]]; }
exists() { [[ -e "$1" || -L "$1" ]]; }
refuse() { echo "Unrecognized path, cleanup stopped: $1" >&2; exit 1; }
for file in "$conf" "$units/$service" "$units/$timer"; do
  exists "$file" || continue
  regular "$file" && [[ $(head -n1 "$file") == '# Managed by LeaguePingo' ]] || refuse "$file"
  echo "Managed configuration: $file"
done
for dir in "$root" "$library"; do
  exists "$dir" || continue
  [[ -d "$dir" && ! -L "$dir" ]] && regular "$dir/$marker" || refuse "$dir"
  echo "Managed directory: $dir"
done
if [[ -d "$root" ]]; then
  id "$user" >/dev/null || refuse "$root"
  for dir in "$root"/{sites,releases}; do
    ! exists "$dir" || [[ -d "$dir" && ! -L "$dir" ]] || refuse "$dir"
  done
fi
if ! "$apply"; then echo 'Inspection only. Use --apply to retire these mirror files.'; exit 0; fi

# Remove the server block only after validating nginx with its configuration excluded.
if regular "$conf"; then
  backup=$(mktemp /etc/nginx/conf.d/.leaguepingo-retire.XXXXXXXX)
  mv -T -- "$conf" "$backup"
  if ! nginx -t || { systemctl is-active --quiet nginx && ! systemctl reload nginx; }; then
    mv -T -- "$backup" "$conf"
    echo 'Nginx configuration restored.' >&2; exit 1
  fi
  rm -- "$backup"
fi
if regular "$units/$timer"; then systemctl disable --now "$timer"; fi
if regular "$units/$service"; then systemctl stop "$service"; fi
rm -f -- "$units/$timer" "$units/$service"
systemctl daemon-reload

if [[ -d "$root" ]]; then
  # Clean data with the updater's own privileges. Unknown entries keep their containing directory.
  runuser -u "$user" -- bash -seu -o pipefail -- "$root" <<'DATA'
cd -P -- "$1"
shopt -s nullglob
[[ ! -L .mirror.lock && ( ! -e .mirror.lock || -f .mirror.lock ) ]]
exec 9>>.mirror.lock
flock -n 9 || { echo 'Updater is still running.' >&2; exit 1; }
plain() { [[ -f "$1" && ! -L "$1" ]]; }
dir() { [[ -d "$1" && ! -L "$1" ]]; }
empty() { [[ -z $(find "$1" -mindepth 1 -maxdepth 1 -print -quit) ]]; }
remove_files() { for f in "$@"; do if plain "$f"; then rm -- "$f"; fi; done; }
finish_dir() {
  local folder=$1 stamp=$2
  if [[ -z $(find "$folder" -mindepth 1 -maxdepth 1 ! -name "$stamp" -print -quit) ]]; then
    rm -- "$folder/$stamp"; rmdir -- "$folder"
  else echo "Retained unrecognized entries in $folder"; fi
}
for link in current previous; do
  if [[ -L "$link" && $(readlink -- "$link") =~ ^sites/[a-f0-9]{40}-[a-f0-9]{16}$ ]]; then rm -- "$link"; fi
done
if dir releases; then
  for d in releases/*; do
    dir "$d" && [[ ${d##*/} =~ ^v[0-9]+\.[0-9]+\.[0-9]+[a-zA-Z0-9.+_-]*$ ]] && plain "$d/.leaguepingo-release.json" || continue
    remove_files "$d"/{leaguepingo.exe,leaguepingo-setup.exe,SHA256SUMS.txt,BUILD-INFO.json,README.txt}
    finish_dir "$d" .leaguepingo-release.json
  done
  if empty releases; then rmdir releases; fi
fi
if dir sites; then
  for d in sites/*; do
    dir "$d" && [[ ${d##*/} =~ ^[a-f0-9]{40}-[a-f0-9]{16}$ ]] && plain "$d/.leaguepingo-mirror.json" || continue
    remove_files "$d"/{index.html,downloads.json,icon.png}
    if dir "$d/media"; then
      remove_files "$d/media"/{overview,review,mayhem,ingame,pregame}.webp "$d/media/SOURCES.md"
      if empty "$d/media"; then rmdir "$d/media"; fi
    fi
    if dir "$d/_astro"; then
      for css in "$d"/_astro/index.*.css; do
        if [[ ${css##*/} =~ ^index\.[a-zA-Z0-9_-]+\.css$ ]]; then remove_files "$css"; fi
      done
      if empty "$d/_astro"; then rmdir "$d/_astro"; fi
    fi
    finish_dir "$d" .leaguepingo-mirror.json
  done
  if empty sites; then rmdir sites; fi
fi
rm -- .mirror.lock
DATA
fi

# Remove known root-owned scripts only if they still match the original installer.
known_script() {
  local file=$1
  shift
  regular "$file" || return 0
  local digest
  digest=$(sha256sum -- "$file")
  for expected in "$@"; do
    if [[ ${digest%% *} == "$expected" ]]; then rm -- "$file"; return 0; fi
  done
  echo "Retained modified script: $file"
}
known_script "$library/mirror.py" ca3239654ca34cdca91da2fe5c3c2a322e161c7a7740752dd2ec155ed90c659f
known_script /tmp/leaguepingo-install.sh bed70b1fde5c6951fbf1523753b92ab693f813ab75f68c12dadb0297a077afee 7e187462877849487eb5e6b73076b5081a46264e75430a882f51c093feadefc9
for dir in "$root" "$library"; do
  [[ -d "$dir" ]] || continue
  if [[ -z $(find "$dir" -mindepth 1 -maxdepth 1 ! -name "$marker" -print -quit) ]]; then
    rm -- "$dir/$marker"; rmdir -- "$dir"
  else echo "Retained files under $dir"; fi
done
echo 'LeaguePingo mirror retirement finished.'
