import copy
import hashlib
import http.server
import io
import json
import os
from pathlib import Path
import tarfile
import tempfile
import threading
import unittest
from unittest.mock import patch

import mirror


def spec(data, url=None):
    value = {'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}
    if url:
        value['url'] = url
    return value


def archive(files, special=None):
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode='w:gz') as bundle:
        for name, data in files.items():
            item = tarfile.TarInfo(name)
            item.size = len(data)
            bundle.addfile(item, io.BytesIO(data))
        if special:
            bundle.addfile(special)
    return output.getvalue()


class Published:
    def __init__(self, letter='a', bad_identity=False):
        commit = letter * 40
        self.tag = f'v0.2.0-dev.g{commit[:8]}'
        prefix = f'{mirror.REPOSITORY}/releases/download/{self.tag}/'
        content = {'leaguepingo.exe': b'MZ portable ' + commit.encode(),
                   'leaguepingo-setup.exe': b'MZ installer ' + commit.encode(), 'README.txt': b'Example usage'}
        receipt = {'schemaVersion': 1, 'platform': 'windows-amd64', 'version': '0.2.0-dev',
                   'commit': 'f' * 40 if bad_identity else commit,
                   'runURL': mirror.REPOSITORY + '/actions/runs/123',
                   'files': [{'name': name, 'bytes': len(content[name]), 'sha256': spec(content[name])['sha256']}
                             for name in ('leaguepingo.exe', 'leaguepingo-setup.exe')]}
        content['BUILD-INFO.json'] = json.dumps(receipt).encode()
        content['SHA256SUMS.txt'] = ('\n'.join(sorted(f'{spec(data)["sha256"]}  {name}'
                                                   for name, data in content.items())) + '\n').encode()
        release = {'version': '0.2.0-dev', 'tag': self.tag, 'commit': commit, 'channel': 'preview',
                   'url': mirror.REPOSITORY + '/releases/tag/' + self.tag, 'buildURL': receipt['runURL'],
                   'files': {name: spec(data, prefix + name) for name, data in content.items()}}
        self.releases = {'schemaVersion': 1, 'stable': None, 'preview': release}
        self.site = {'index.html': b'<html><a href="/releases/' + self.tag.encode() + b'/leaguepingo-setup.exe">Download</a></html>',
                     'downloads.json': json.dumps(mirror.local_downloads(self.releases)).encode(),
                     '_astro/site.css': b'body { color: white; }'}
        self.archive = archive(self.site)
        self.manifest = {'schemaVersion': 1, 'websiteCommit': commit,
                         'archive': {'name': 'mirror.tar.gz', **spec(self.archive)}, 'releases': self.releases}
        self.content = content
        self.calls = []
        self.override = {}

    def fetch(self, url, target, expected=None, limit=mirror.MAX_FILE):
        self.calls.append(url)
        content = {mirror.SOURCE + 'mirror.json': json.dumps(self.manifest).encode(),
                   mirror.SOURCE + 'mirror.tar.gz': self.archive,
                   **{self.releases['preview']['files'][name]['url']: data for name, data in self.content.items()},
                   **self.override}
        target.write_bytes(content[url])


class MirrorTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)

    def publish(self, published):
        return mirror.sync(self.root, fetcher=published.fetch)

    def test_first_publish_and_repeated_update_only_fetches_small_manifest(self):
        source = Published()
        self.assertEqual(self.publish(source), (True, source.tag))
        self.assertIn(b'/releases/', (self.root / 'current/index.html').read_bytes())
        folder = self.root / 'releases' / source.tag
        self.assertEqual((folder / 'leaguepingo.exe').read_bytes(), source.content['leaguepingo.exe'])
        self.assertEqual(folder.stat().st_mode & 0o777, 0o755)
        self.assertEqual((folder / 'leaguepingo.exe').stat().st_mode & 0o777, 0o644)
        source.calls.clear()
        self.assertEqual(self.publish(source), (False, source.tag))
        self.assertEqual(source.calls, [mirror.SOURCE + 'mirror.json'])

    def test_bad_binary_never_advances_current_or_publishes_partial_release(self):
        self.publish(Published())
        previous = (self.root / 'current').readlink()
        source = Published('b')
        source.override[source.releases['preview']['files']['leaguepingo.exe']['url']] = b'tampered'
        with self.assertRaisesRegex(ValueError, 'checksum'):
            self.publish(source)
        self.assertEqual((self.root / 'current').readlink(), previous)
        self.assertFalse((self.root / 'releases' / source.tag).exists())
        self.assertFalse(list(self.root.glob('.sync-*')))

    def test_restrictive_administrator_umask_still_publishes_nginx_readable_files(self):
        previous = os.umask(0o077)
        try:
            source = Published()
            self.publish(source)
        finally:
            os.umask(previous)
        for folder in (self.root / 'sites', self.root / 'current', self.root / 'current/_astro',
                       self.root / 'releases', self.root / 'releases' / source.tag):
            self.assertEqual(folder.stat().st_mode & 0o777, 0o755)
        self.assertEqual((self.root / 'current/index.html').stat().st_mode & 0o777, 0o644)

    def test_wrong_build_identity_rejected_even_when_all_file_hashes_match(self):
        with self.assertRaisesRegex(ValueError, 'Build identity'):
            self.publish(Published(bad_identity=True))
        self.assertFalse((self.root / 'current').exists())

    def test_archive_checksum_and_inconsistent_page_links_preserve_previous(self):
        self.publish(Published())
        previous = (self.root / 'current').readlink()
        source = Published('b')
        source.override[mirror.SOURCE + 'mirror.tar.gz'] = b'not the archive'
        with self.assertRaisesRegex(ValueError, 'archive checksum'):
            self.publish(source)
        self.assertEqual((self.root / 'current').readlink(), previous)
        source.override.clear()
        source.archive = archive({**source.site, 'downloads.json': b'{}'})
        source.manifest['archive'].update(spec(source.archive))
        with self.assertRaisesRegex(ValueError, 'download links'):
            self.publish(source)
        self.assertEqual((self.root / 'current').readlink(), previous)

    def test_foreign_origins_paths_and_invalid_metadata_rejected(self):
        mutations = [lambda m: m['archive'].update(name='../escape.tar.gz'),
                     lambda m: m['releases']['preview'].update(tag='../escape'),
                     lambda m: m['releases']['preview']['files']['leaguepingo.exe'].update(url='https://example.com/payload'),
                     lambda m: m['releases']['preview']['files']['leaguepingo.exe'].update(size=-1),
                     lambda m: m['releases']['preview'].update(commit='invalid'),
                     lambda m: m['releases']['preview'].update(channel='stable')]
        for mutation in mutations:
            with self.subTest(mutation=mutation):
                data = copy.deepcopy(Published().manifest)
                mutation(data)
                with self.assertRaises(ValueError):
                    mirror.validate_manifest(data)

    def test_archive_traversal_symlinks_hardlinks_and_hidden_files_rejected(self):
        for name, kind in (('../escape', tarfile.REGTYPE), ('/escape', tarfile.REGTYPE),
                           ('link', tarfile.SYMTYPE), ('link', tarfile.LNKTYPE), ('.hidden', tarfile.REGTYPE)):
            with self.subTest(name=name, kind=kind):
                item = tarfile.TarInfo(name)
                item.type = kind
                item.linkname = '../escape'
                bundle = self.root / 'test.tar.gz'
                bundle.write_bytes(archive({}, item))
                with self.assertRaises(ValueError):
                    mirror.extract_site(bundle, self.root / 'extracted')
        self.assertFalse((self.root.parent / 'escape').exists())

    def test_two_versions_retained_unknown_files_preserved_and_rollback_is_offline(self):
        first, second, third = Published(), Published('b'), Published('c')
        self.publish(first)
        self.publish(second)
        personal = self.root / 'releases' / 'personal-files'
        personal.mkdir()
        (personal / 'notes.txt').write_text('retain me')
        self.publish(third)
        self.assertFalse((self.root / 'releases' / first.tag).exists())
        self.assertTrue((self.root / 'releases' / second.tag).is_dir())
        self.assertTrue((personal / 'notes.txt').is_file())
        self.assertEqual(len(list((self.root / 'sites').iterdir())), 2)
        mirror.rollback(self.root)
        self.assertIn('b' * 40, str((self.root / 'current').readlink()))
        self.assertIn('c' * 40, str((self.root / 'previous').readlink()))

    def test_archive_entry_count_and_expansion_are_bounded(self):
        target = self.root / 'archive.tar.gz'
        target.write_bytes(archive({str(index): b'' for index in range(513)}))
        with self.assertRaisesRegex(ValueError, 'Too many'):
            mirror.extract_site(target, self.root / 'many-files')
        target.write_bytes(archive({'large.txt': b'a' * (1024 * 1024 + 8192)}))
        with patch('mirror.MAX_SITE', 4096), self.assertRaisesRegex(ValueError, 'exceeds limit'):
            mirror.extract_site(target, self.root / 'large')

    def test_reuse_checks_cached_binary_and_symlinks_are_refused(self):
        source = Published()
        self.publish(source)
        binary = self.root / 'releases' / source.tag / 'leaguepingo.exe'
        binary.write_bytes(b'tampered')
        with self.assertRaisesRegex(ValueError, 'checksum'):
            self.publish(source)
        (self.root / 'current').unlink()
        (self.root / 'current').symlink_to('/tmp')
        with self.assertRaisesRegex(ValueError, 'symlink'):
            mirror.current_site(self.root)

    def test_low_disk_space_does_not_download_binaries_or_replace_current(self):
        source = Published()
        with patch('mirror.shutil.disk_usage') as usage:
            usage.return_value.free = 0
            with self.assertRaisesRegex(ValueError, 'disk space'):
                self.publish(source)
        self.assertEqual(source.calls, [mirror.SOURCE + 'mirror.json'])

    def test_cleanup_error_is_reported_without_claiming_publication_failed(self):
        with patch('mirror.clean_previous_versions', side_effect=OSError('cleanup unavailable')):
            self.assertTrue(self.publish(Published())[0])
        self.assertTrue((self.root / 'current/index.html').is_file())

    def test_real_http_download_checks_stream_size_and_hash(self):
        payload = b'MZ example installer'

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                self.send_response(200)
                self.end_headers()
                self.wfile.write(payload)

            def log_message(self, *_):
                pass

        server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Handler)
        worker = threading.Thread(target=server.serve_forever, daemon=True)
        worker.start()
        try:
            url = f'http://127.0.0.1:{server.server_port}/installer.exe'
            target = self.root / 'installer.exe'
            mirror.fetch(url, target, spec(payload))
            self.assertEqual(target.read_bytes(), payload)
            with self.assertRaisesRegex(ValueError, 'checksum'):
                mirror.fetch(url, target, {**spec(payload), 'sha256': '0' * 64})
            with self.assertRaisesRegex(ValueError, 'exceeds'):
                mirror.fetch(url, target, {**spec(payload), 'size': 1})
            with self.assertRaisesRegex(ValueError, 'HTTPS'):
                mirror.fetch('http://example.com/installer.exe', target)
        finally:
            server.shutdown()
            server.server_close()
            worker.join()


if __name__ == '__main__':
    unittest.main()
