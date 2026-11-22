#!/usr/bin/env python3
"""Publish a verified static website and release files without a server build toolchain."""
import argparse
import copy
import fcntl
import gzip
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import sys
import tarfile
import tempfile
import urllib.parse
import urllib.request

SOURCE = 'https://zjing1125.github.io/LeaguePingo/'
REPOSITORY = 'https://github.com/ZJing1125/LeaguePingo'
FILES = ('BUILD-INFO.json', 'SHA256SUMS.txt', 'README.txt', 'leaguepingo.exe', 'leaguepingo-setup.exe')
MARKER = '.leaguepingo-mirror.json'
RELEASE_MARKER = '.leaguepingo-release.json'
MAX_FILE = 256 * 1024 * 1024
MAX_SITE = 32 * 1024 * 1024


def checked(condition, message):
    if not condition:
        raise ValueError(message)


def digest(file):
    with Path(file).open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def matches(file, spec):
    return (file.is_file() and not file.is_symlink() and file.stat().st_size == spec['size']
            and digest(file) == spec['sha256'])


def validate_spec(spec, limit):
    checked(isinstance(spec, dict) and type(spec.get('size')) is int and 0 < spec['size'] <= limit
            and isinstance(spec.get('sha256'), str) and re.fullmatch('[a-f0-9]{64}', spec['sha256']),
            'Invalid file size or SHA-256')


def validate_manifest(data):
    checked(isinstance(data, dict) and data.get('schemaVersion') == 1, 'Invalid mirror manifest')
    checked(re.fullmatch('[a-f0-9]{40}', str(data.get('websiteCommit', ''))), 'Invalid website commit')
    validate_spec(data.get('archive'), MAX_SITE)
    checked(data['archive'].get('name') == 'mirror.tar.gz', 'Unexpected website archive')
    releases = data.get('releases')
    checked(isinstance(releases, dict) and releases.get('schemaVersion') == 1, 'Invalid release manifest')
    checked(releases.get('stable') or releases.get('preview'), 'No published release')
    for key in ('stable', 'preview'):
        release = releases.get(key)
        if release is None:
            continue
        checked(isinstance(release, dict) and release.get('channel') == key, 'Invalid release channel')
        tag, commit = release.get('tag', ''), release.get('commit', '')
        checked(isinstance(tag, str) and re.fullmatch(r'[a-zA-Z0-9][\w.+-]{0,120}', tag), 'Invalid release tag')
        checked(isinstance(commit, str) and re.fullmatch('[a-f0-9]{40}', commit), 'Invalid release commit')
        checked(re.fullmatch(r'\d+\.\d+\.\d+(?:-[\w.-]+)?', str(release.get('version', ''))), 'Invalid version')
        checked(tag in (f"v{release['version']}", f"v{release['version']}.g{commit[:8]}"), 'Version tag mismatch')
        checked(release.get('url') == f'{REPOSITORY}/releases/tag/{tag}', 'Unexpected release origin')
        checked(re.fullmatch(re.escape(REPOSITORY) + r'/actions/runs/\d+', str(release.get('buildURL', ''))),
                'Unexpected build origin')
        checked(isinstance(release.get('files'), dict) and set(release['files']) == set(FILES), 'Unexpected release files')
        for name, spec in release['files'].items():
            validate_spec(spec, MAX_FILE)
            checked(spec.get('url') == f'{REPOSITORY}/releases/download/{tag}/{name}', 'Unexpected asset origin')
    return data


def local_downloads(data):
    result = copy.deepcopy(data)
    for key in ('stable', 'preview'):
        release = result.get(key)
        if release:
            for name, spec in release['files'].items():
                spec['url'] = f"/releases/{release['tag']}/{name}"
    return result


def fetch(url, target, spec=None, limit=MAX_FILE):
    parsed = urllib.parse.urlsplit(url)
    checked(parsed.scheme == 'https' or (parsed.scheme == 'http' and parsed.hostname == '127.0.0.1'),
            'Downloads require HTTPS')
    request = urllib.request.Request(url, headers={'User-Agent': 'LeaguePingo-mirror/1'})
    count, checksum = 0, hashlib.sha256()
    with urllib.request.urlopen(request, timeout=45) as response, target.open('wb') as output:
        checked(urllib.parse.urlsplit(response.url).scheme == parsed.scheme, 'Unexpected download protocol')
        while chunk := response.read(256 * 1024):
            count += len(chunk)
            checked(count <= min(limit, spec['size'] if spec else limit), 'Download exceeds expected size')
            checksum.update(chunk)
            output.write(chunk)
    if spec:
        checked(count == spec['size'] and checksum.hexdigest() == spec['sha256'], 'Download checksum mismatch')


def verify_release(folder, release):
    for name, spec in release['files'].items():
        checked(matches(folder / name, spec), f'Release checksum mismatch: {name}')
    info = json.loads((folder / 'BUILD-INFO.json').read_text())
    checked(isinstance(info, dict), 'Invalid build receipt')
    checked(info.get('schemaVersion') == 1 and info.get('platform') == 'windows-amd64'
            and info.get('commit') == release['commit'] and info.get('version') == release['version']
            and info.get('runURL') == release['buildURL'], 'Build identity mismatch')
    binaries = info.get('files')
    checked(isinstance(binaries, list) and len(binaries) == 2 and all(isinstance(item, dict) for item in binaries),
            'Invalid binary receipt')
    for name in ('leaguepingo.exe', 'leaguepingo-setup.exe'):
        spec = release['files'][name]
        checked(sum(item.get('name') == name and item.get('bytes') == spec['size']
                    and item.get('sha256') == spec['sha256'] for item in binaries) == 1, 'Binary identity mismatch')
    expected = sorted(f"{release['files'][name]['sha256']}  {name}" for name in FILES if name != 'SHA256SUMS.txt')
    checked(sorted((folder / 'SHA256SUMS.txt').read_text().strip().splitlines()) == expected, 'Release checksum list differs')


def prepare_release(root, release, fetcher):
    target = root / 'releases' / release['tag']
    checked(not target.is_symlink(), 'Release directory cannot be a symlink')
    if target.exists():
        checked(json.loads((target / RELEASE_MARKER).read_text()) == release, 'Existing release identity differs')
        verify_release(target, release)
        return
    with tempfile.TemporaryDirectory(prefix='.download-', dir=root) as temporary:
        stage = Path(temporary)
        for name, spec in release['files'].items():
            fetcher(spec['url'], stage / name, spec)
            (stage / name).chmod(0o644)
        verify_release(stage, release)
        (stage / RELEASE_MARKER).write_text(json.dumps(release))
        target.parent.mkdir(exist_ok=True)
        target.parent.chmod(0o755)
        # TemporaryDirectory is private; the published release must be readable by nginx.
        stage.chmod(0o755)
        stage.rename(target)


def extract_site(archive, destination):
    with gzip.open(archive, 'rb') as source, tempfile.TemporaryFile(dir=archive.parent) as expanded:
        size = 0
        while chunk := source.read(256 * 1024):
            size += len(chunk)
            checked(size <= MAX_SITE + 1024 * 1024, 'Expanded website exceeds limit')
            expanded.write(chunk)
        expanded.seek(0)
        with tarfile.open(fileobj=expanded, mode='r:') as bundle:
            extract_members(bundle, destination)
    for directory, _, _ in os.walk(destination):
        Path(directory).chmod(0o755)


def extract_members(bundle, destination):
    count, size = 0, 0
    for member in bundle:
        count += 1
        size += member.size
        checked(count <= 512, 'Too many website entries')
        checked(size <= MAX_SITE, 'Expanded website exceeds limit')
        name = PurePosixPath(member.name)
        checked(not name.is_absolute() and '..' not in name.parts and '\\' not in member.name,
                'Unsafe website path')
        checked(member.isfile() or member.isdir(), 'Website links and special files are forbidden')
        checked(not any(part.startswith('.') for part in name.parts), 'Hidden website files are forbidden')
        target = destination.joinpath(*name.parts)
        if member.isdir():
            target.mkdir(parents=True, exist_ok=True)
        else:
            checked(member.size <= MAX_SITE, 'Website file exceeds limit')
            target.parent.mkdir(parents=True, exist_ok=True)
            with bundle.extractfile(member) as source, target.open('wb') as output:
                shutil.copyfileobj(source, output, 256 * 1024)
            target.chmod(0o644)


def verify_site(site, data):
    checked(not site.is_symlink() and (site / 'index.html').is_file(), 'Incomplete website')
    checked(json.loads((site / 'downloads.json').read_text()) == local_downloads(data['releases']),
            'Website download links do not match releases')


def current_site(root, name='current'):
    link = root / name
    if not link.is_symlink():
        checked(not link.exists(), f'{name} must be a managed symlink')
        return None
    target = Path(os.readlink(link))
    checked(not target.is_absolute() and len(target.parts) == 2 and target.parts[0] == 'sites'
            and re.fullmatch('[a-f0-9]{40}-[a-f0-9]{16}', target.parts[1]), 'Unexpected website symlink')
    return root / target


def activate(root, site):
    previous = current_site(root)
    if previous == site:
        return False
    for name, target in (('previous', previous), ('current', site)):
        if target:
            current_site(root, name)
            with tempfile.NamedTemporaryFile(prefix='.' + name + '-', dir=root, delete=False) as handle:
                temporary = Path(handle.name)
            try:
                temporary.unlink()
                temporary.symlink_to(target.relative_to(root))
                temporary.replace(root / name)
            finally:
                temporary.unlink(missing_ok=True)
    return True


def clean_previous_versions(root):
    keep = {site for site in (current_site(root), current_site(root, 'previous')) if site}
    tags = set()
    for site in keep:
        data = validate_manifest(json.loads((site / MARKER).read_text()))
        tags.update(release['tag'] for key in ('stable', 'preview') if (release := data['releases'].get(key)))
    for site in (root / 'sites').iterdir():
        if site not in keep and not site.is_symlink() and (site / MARKER).is_file():
            shutil.rmtree(site)
    for folder in (root / 'releases').iterdir():
        # Only remove a complete, self-identifying release produced by this updater.
        if folder.name in tags or folder.is_symlink() or not folder.is_dir():
            continue
        if set(item.name for item in folder.iterdir()) == set(FILES) | {RELEASE_MARKER}:
            release = json.loads((folder / RELEASE_MARKER).read_text())
            if release.get('tag') == folder.name and release.get('url') == f'{REPOSITORY}/releases/tag/{folder.name}':
                shutil.rmtree(folder)


def sync(root, source=SOURCE, fetcher=fetch):
    with tempfile.TemporaryDirectory(prefix='.sync-', dir=root) as temporary:
        stage = Path(temporary)
        manifest_file = stage / 'mirror.json'
        fetcher(source.rstrip('/') + '/mirror.json', manifest_file, limit=128 * 1024)
        data = validate_manifest(json.loads(manifest_file.read_text()))
        needed = data['archive']['size'] + MAX_SITE + 64 * 1024 * 1024
        for key in ('stable', 'preview'):
            release = data['releases'].get(key)
            if release and not (root / 'releases' / release['tag']).exists():
                needed += sum(spec['size'] for spec in release['files'].values())
        checked(shutil.disk_usage(root).free >= needed, 'Insufficient disk space; current website preserved')
        for key in ('stable', 'preview'):
            if release := data['releases'].get(key):
                prepare_release(root, release, fetcher)
        identity = data['websiteCommit'] + '-' + data['archive']['sha256'][:16]
        sites = root / 'sites'
        sites.mkdir(exist_ok=True)
        sites.chmod(0o755)
        site = sites / identity
        checked(not site.is_symlink(), 'Website directory cannot be a symlink')
        if site.exists():
            checked(json.loads((site / MARKER).read_text()) == data, 'Existing website identity differs')
        else:
            archive = stage / 'mirror.tar.gz'
            fetcher(source.rstrip('/') + '/mirror.tar.gz', archive, data['archive'], limit=MAX_SITE)
            checked(matches(archive, data['archive']), 'Website archive checksum mismatch')
            extracted = stage / 'site'
            extracted.mkdir()
            extract_site(archive, extracted)
            verify_site(extracted, data)
            (extracted / MARKER).write_text(json.dumps(data))
            extracted.rename(site)
        verify_site(site, data)
        changed = activate(root, site)
        try:
            clean_previous_versions(root)
        except (ValueError, OSError, KeyError, TypeError) as error:
            print('Mirror is active; old-version cleanup skipped: ' + str(error), file=sys.stderr)
        selected = data['releases'].get('stable') or data['releases']['preview']
        return changed, selected['tag']


def rollback(root):
    site = current_site(root, 'previous')
    checked(site is not None and site.is_dir(), 'No previous website available')
    data = validate_manifest(json.loads((site / MARKER).read_text()))
    verify_site(site, data)
    for key in ('stable', 'preview'):
        if release := data['releases'].get(key):
            verify_release(root / 'releases' / release['tag'], release)
    activate(root, site)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=('sync', 'rollback'))
    parser.add_argument('--root', default='/srv/leaguepingo')
    args = parser.parse_args()
    root = Path(args.root).resolve(strict=True)
    checked((root / '.leaguepingo-managed').is_file(), 'Deployment root is not marked as managed')
    with (root / '.mirror.lock').open('a') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if args.action == 'rollback':
            rollback(root)
            print('Previous verified website restored')
        else:
            changed, tag = sync(root)
            print(('Published' if changed else 'Already current') + ': ' + tag)


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, KeyError, TypeError, tarfile.TarError) as error:
        raise SystemExit('Mirror update failed; website was not advanced: ' + str(error))
