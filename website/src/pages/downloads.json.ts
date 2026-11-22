import data from '../data/releases.json';
export function GET() { return new Response(JSON.stringify(data), {headers: {'Content-Type':'application/json; charset=utf-8'}}); }
