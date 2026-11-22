import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {readFile, writeFile} from 'node:fs/promises';
import {test} from 'node:test';
import {origin, publishDownloads, uploader} from '../scripts/cos.mjs';

const tag = 'v1.2.3-dev.gaaaaaaaa';
const github = 'https://github.com/ZJing1125/LeaguePingo/releases/download';
const names = ['leaguepingo.exe', 'leaguepingo-setup.exe', 'BUILD-INFO.json', 'SHA256SUMS.txt', 'README.txt'];
const hash = (algorithm, bytes) => createHash(algorithm).update(bytes).digest('hex');
const bytes = name => Buffer.from('fixture asset ' + name);
function setup({multipart = false} = {}) {
  const files = Object.fromEntries(names.map(name => [name, {
    url: `${github}/${tag}/${name}`, size: bytes(name).length, sha256: hash('sha256', bytes(name)),
  }]));
  const data = {schemaVersion: 1, stable: null, preview: {tag, files}};
  const objects = new Map();
  const io = {
    getFile: (url, file) => writeFile(file, bytes(new URL(url).pathname.split('/').pop())),
    upload: async (key, file, spec, md5) => {
      const body = await readFile(file);
      assert.equal(hash('md5', body), md5);
      objects.set(origin + '/' + key, body);
    },
    request: async (url, init) => {
      assert.equal(init.redirect, 'error');
      const body = objects.get(url);
      if (!body) return new Response(null, {status: 404});
      if (!init.headers.Range) return new Response(body);
      return new Response(body.subarray(0, 1), {status: 206, headers: {
        'content-range': `bytes 0-0/${body.length}`, etag: multipart ? '"sample-9"' : `"${hash('md5', body)}"`,
      }});
    },
  };
  return {data, objects, io};
}

test('first publication and credential-free rerun use immutable public URLs for all files', async () => {
  const {data, objects, io} = setup();
  const result = await publishDownloads(data, io);
  assert.equal(objects.size, 5);
  assert.deepEqual(await publishDownloads(data, {...io, upload: null}), result);
  for (const name of names) {
    assert.deepEqual(result.preview.files[name], {...data.preview.files[name], url: `${origin}/dev/${tag}/${name}`});
    assert.ok(data.preview.files[name].url.startsWith(github));
  }
});
test('multipart objects are verified from their public bytes and a collision is never overwritten', async () => {
  const {data, objects, io} = setup({multipart: true});
  await publishDownloads(data, io);
  const url = `${origin}/dev/${tag}/leaguepingo.exe`;
  const bad = Buffer.alloc(objects.get(url).length);
  objects.set(url, bad);
  await assert.rejects(publishDownloads(data, io), /checksum/);
  assert.deepEqual(objects.get(url), bad);
});
test('refuses inaccessible, missing and unsigned-source-mismatched files before publishing links', async () => {
  const {data, io} = setup();
  await assert.rejects(publishDownloads(data, {...io, upload: null}), /Missing dev/);
  await assert.rejects(publishDownloads(data, {...io, request: async () => new Response(null, {status: 403})}), /403/);
  await assert.rejects(publishDownloads(data, {...io, upload: async () => {}}), /still unavailable/);
  await assert.rejects(publishDownloads(data, {...io, getFile: (_, file) => writeFile(file, 'wrong')}), /checksum/);
  data.preview.files['leaguepingo.exe'].url = 'https://example.org/asset';
  await assert.rejects(publishDownloads(data, io), /metadata/);
});
test('publishing credentials are restricted to the trusted dev step before Pages builds', async () => {
  const site = await readFile(new URL('../../.github/workflows/website.yml', import.meta.url), 'utf8');
  const step = site.slice(site.indexOf('- name: Publish and verify public COS downloads'), site.indexOf('- name: Build static website'));
  assert.match(step, /github.ref == 'refs\/heads\/dev' && github.event_name != 'pull_request'/);
  assert.match(step, /secrets.COS_SECRET_KEY/);
  assert.ok(site.indexOf('run sync-release') < site.indexOf(step));
  assert.equal(site.match(/secrets.COS_SECRET_KEY/g)?.length, 1);
  assert.doesNotMatch(site, /build:mirror|mirror\.tar|secrets: inherit/);
});
test('signed uploads keep credentials out of process arguments and use bounded HTTPS transport', async () => {
  const env = {COS_AUTO_UPLOAD: 'true', COS_SECRET_ID: 'fixture-id', COS_SECRET_KEY: 'fixture-key'};
  const upload = uploader(env, (command, args, options) => {
    assert.equal(command, 'curl');
    assert.equal(args.at(-1), `${origin}/dev/${tag}/leaguepingo.exe`);
    assert.ok(args.includes('--max-time'));
    assert.ok(args.includes('--connect-timeout'));
    assert.ok(args.includes('--speed-time'));
    assert.ok(args.includes('--fail'));
    assert.ok(args.includes('--http1.1'));
    assert.equal(args.includes('--location'), false);
    assert.ok(!args.join(' ').includes(env.COS_SECRET_ID));
    assert.ok(!args.join(' ').includes(env.COS_SECRET_KEY));
    assert.match(options.input, /Authorization: q-sign-algorithm=sha1/);
    assert.match(options.input, /x-cos-forbid-overwrite: true/);
    assert.match(options.input, /content-md5:/);
    assert.ok(!options.input.includes(env.COS_SECRET_KEY));
    return '{"status":200,"bytes":7,"seconds":0.1,"speed":70,"exit":0}';
  });
  await upload(`dev/${tag}/leaguepingo.exe`, '/fixture.exe', {size: 7}, 'a'.repeat(32));
  const conflict = uploader(env, () => {throw {stdout: '{"status":409,"bytes":0,"seconds":0.1,"speed":0,"exit":22}'};});
  await conflict(`dev/${tag}/leaguepingo.exe`, '/fixture.exe', {size: 7}, 'a'.repeat(32));
  const denied = uploader(env, () => {throw {stdout: '{"status":403,"bytes":0,"seconds":0.1,"speed":0,"exit":22}', message: env.COS_SECRET_KEY};});
  await assert.rejects(denied(`dev/${tag}/leaguepingo.exe`, '/fixture.exe', {size: 7}, 'a'.repeat(32)), /HTTP 403/);
  const timeout = uploader(env, () => {throw {stdout: '{"status":"000","bytes":5,"seconds":120,"speed":0,"exit":28}'};});
  await assert.rejects(timeout(`dev/${tag}/leaguepingo.exe`, '/fixture.exe', {size: 7}, 'a'.repeat(32)), /curl 28, uploaded 5 bytes/);
});
test('manual publication only verifies files until automatic upload is explicitly enabled', () => {
  const credentials = {COS_SECRET_ID: 'fixture-id', COS_SECRET_KEY: 'fixture-key'};
  assert.equal(uploader(credentials), null);
  assert.equal(uploader({}), null);
  assert.throws(() => uploader({COS_AUTO_UPLOAD: 'true'}), /Both COS upload secrets/);
});
