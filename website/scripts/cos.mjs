import {createHash} from 'node:crypto';
import {execFileSync} from 'node:child_process';
import {createReadStream} from 'node:fs';
import {mkdtemp, readFile, rename, rm, writeFile} from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import COS from 'cos-nodejs-sdk-v5';
import {repository} from './releases.mjs';

const bucket = 'pingo-1538719098';
const region = 'ap-shanghai';
export const origin = `https://${bucket}.cos.${region}.myqcloud.com`;
const names = ['BUILD-INFO.json', 'SHA256SUMS.txt', 'README.txt', 'leaguepingo-setup.exe', 'leaguepingo.exe'];
const releaseOrigin = `https://github.com/${repository}/releases/download`;

async function checksum(stream, expected) {
  const sha = createHash('sha256'), md5 = createHash('md5');
  let bytes = 0;
  for await (const chunk of stream) {
    bytes += chunk.length;
    if (bytes > expected.size) throw new Error('File exceeds release size');
    sha.update(chunk);
    md5.update(chunk);
  }
  if (bytes !== expected.size || sha.digest('hex') !== expected.sha256) throw new Error('Release checksum mismatch');
  return md5.digest('hex');
}

async function publicMatch(url, spec, md5, request) {
  const get = headers => request(url, {headers, redirect: 'error', signal: AbortSignal.timeout(120_000)});
  const part = await get({Range: 'bytes=0-0'});
  if (part.status === 200) {
    await checksum(part.body, spec);
    return true;
  }
  if (part.status !== 206) {
    await part.body?.cancel();
    if (part.status === 404) return false;
    throw new Error(`Anonymous COS read returned HTTP ${part.status}`);
  }
  if (part.headers.get('content-range') !== `bytes 0-0/${spec.size}` || (await part.arrayBuffer()).byteLength !== 1)
    throw new Error('COS object size differs');
  // COS single-part PUTs return the content MD5. Multipart and encrypted objects may use other ETags.
  if (part.headers.get('etag') === `"${md5}"`) return true;
  const whole = await get({});
  if (whole.status !== 200) {
    await whole.body?.cancel();
    throw new Error(`COS verification returned HTTP ${whole.status}`);
  }
  await checksum(whole.body, spec);
  return true;
}

function download(url, file, size) {
  console.log('Downloading release file:', path.basename(file));
  execFileSync('curl', ['--fail', '--location', '--silent', '--show-error', '--proto', '=https',
    '--proto-redir', '=https', '--max-time', '180', '--retry', '2', '--max-filesize', String(size),
    '--output', file, url], {stdio: ['ignore', 'ignore', 'inherit']});
  console.log('Release file downloaded:', path.basename(file));
}

export function uploader(env, execute = execFileSync) {
  if (env.COS_AUTO_UPLOAD !== 'true') return null;
  if (!env.COS_SECRET_ID || !env.COS_SECRET_KEY) throw new Error('Both COS upload secrets are required');
  return async (key, file, spec, md5) => {
    const url = new URL(`${origin}/${key}`);
    const headers = {
      host: url.host,
      'content-length': String(spec.size),
      'content-md5': Buffer.from(md5, 'hex').toString('base64'),
      'content-type': 'application/octet-stream',
      'content-disposition': 'attachment',
      'cache-control': 'public, max-age=31536000, immutable',
      'x-cos-forbid-overwrite': 'true',
    };
    const authorization = COS.getAuthorization({SecretId: env.COS_SECRET_ID, SecretKey: env.COS_SECRET_KEY,
      Method: 'PUT', Pathname: url.pathname, Headers: headers, Expires: 11_700});
    // Supply authorization through stdin so it stays out of process arguments and files.
    const config = Object.entries({...headers, Authorization: authorization})
      .map(([name, value]) => `header = ${JSON.stringify(`${name}: ${value}`)}`).join('\n') + '\nheader = "Expect:"\n';
    console.log('Uploading COS object:', key);
    let result;
    try {
      result = execute('curl', ['--config', '-', '--fail', '--silent', '--show-error', '--http1.1',
        '--proto', '=https', '--connect-timeout', '15', '--max-time', '5400', '--retry', '1',
        '--retry-delay', '5', '--speed-time', '120', '--speed-limit', '1024', '--upload-file', file,
        '--output', os.devNull, '--write-out',
        '{"status":"%{http_code}","bytes":%{size_upload},"seconds":%{time_total},"speed":%{speed_upload},"exit":%{exitcode}}', url.href],
      {input: config, encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe']}).trim();
    } catch (error) {
      result = String(error.stdout || '').trim();
    }
    let transfer;
    try { transfer = JSON.parse(result); } catch { transfer = {status: 0, exit: 'unknown', bytes: 0}; }
    transfer.status = Number(transfer.status);
    console.log('COS transfer:', JSON.stringify(transfer));
    if (transfer.status !== 200 && transfer.status !== 409)
      throw new Error(`COS upload failed for ${key} (HTTP ${transfer.status}, curl ${transfer.exit}, uploaded ${transfer.bytes} bytes)`);
    // A concurrent publisher may have won the write. Its public bytes are checked below.
    console.log(transfer.status === 200 ? 'COS object uploaded:' : 'COS object already exists:', key);
  };
}

export async function publishDownloads(data, {request = fetch, getFile = download, upload = null} = {}) {
  const published = structuredClone(data);
  const dir = await mkdtemp(path.join(os.tmpdir(), 'pingo-cos-'));
  try {
    for (const release of [published.stable, published.preview].filter(Boolean)) {
      if (!/^v\d+\.\d+\.\d+[\w.-]*$/.test(release.tag) || Object.keys(release.files).length !== names.length)
        throw new Error('Invalid release tag or file list');
      for (const name of names) {
        const file = path.join(dir, name), spec = release.files[name];
        if (spec?.url !== `${releaseOrigin}/${release.tag}/${name}` || !Number.isSafeInteger(spec.size) ||
            spec.size <= 0 || !/^[a-f0-9]{64}$/.test(spec.sha256)) throw new Error('Invalid release file metadata');
        const key = `dev/${release.tag}/${name}`, url = `${origin}/${key}`;
        await getFile(spec.url, file, spec.size);
        const md5 = await checksum(createReadStream(file), spec);
        if (!await publicMatch(url, spec, md5, request)) {
          if (!upload) throw new Error(`Missing ${key}. Upload this release file to COS, then rerun the website workflow. Optional automatic upload requires COS_AUTO_UPLOAD=true and both COS upload secrets.`);
          await upload(key, file, spec, md5);
          if (!await publicMatch(url, spec, md5, request)) throw new Error(`COS object still unavailable: ${key}`);
        }
        spec.url = url;
      }
    }
    return published;
  } finally {
    await rm(dir, {recursive: true, force: true});
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const manifest = fileURLToPath(new URL('../src/data/releases.json', import.meta.url));
  try {
    const data = await publishDownloads(JSON.parse(await readFile(manifest, 'utf8')), {upload: uploader(process.env)});
    await writeFile(manifest + '.tmp', JSON.stringify(data, null, 2) + '\n');
    await rename(manifest + '.tmp', manifest);
    console.log('Public COS download files verified for', (data.stable ?? data.preview).tag);
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
