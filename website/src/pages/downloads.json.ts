import data from '../data/releases.json';
import { localDownloads } from '../../scripts/mirror-bundle.mjs';
export function GET() {
  const downloads = import.meta.env.WEBSITE_MIRROR === '1' ? localDownloads(data) : data;
  return new Response(JSON.stringify(downloads), {headers: {'Content-Type':'application/json; charset=utf-8'}});
}
