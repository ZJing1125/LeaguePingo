import {test} from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import {createHash} from 'node:crypto';
import {manifest,selectChannels} from '../scripts/releases.mjs';
const hash=b=>createHash('sha256').update(b).digest('hex');
const base='https://github.com/ZJing1125/LeaguePingo';
function fixture(){
 const commit='a'.repeat(40),version='0.2.0-dev',tag=`v${version}.g${commit.slice(0,8)}`;
 const content={'leaguepingo.exe':Buffer.from('MZ exe'),'leaguepingo-setup.exe':Buffer.from('MZ installer'),'README.txt':Buffer.from('demo')};
 content['BUILD-INFO.json']=Buffer.from(JSON.stringify({schemaVersion:1,version,commit,platform:'windows-amd64',runURL:base+'/actions/runs/123',files:['leaguepingo.exe','leaguepingo-setup.exe'].map(name=>({name,bytes:content[name].length,sha256:hash(content[name])}))}));
 content['SHA256SUMS.txt']=Buffer.from(Object.entries(content).map(([n,b])=>`${hash(b)}  ${n}`).join('\n')+'\n');
 const release={tag_name:tag,target_commitish:commit,html_url:base+'/releases/tag/'+tag,prerelease:true,draft:false,published_at:'2026-11-17T00:00:00Z',assets:Object.entries(content).map(([name,b])=>({name,size:b.length,digest:'sha256:'+hash(b),state:'uploaded',browser_download_url:base+'/releases/download/'+tag+'/'+name}))};
 return{release,content};
}
test('selects channels by publication time, excluding drafts, retaining preview-only downloads',()=>{
 const preview={prerelease:true,published_at:'2026-11-17T00:00:00Z'},stable={prerelease:false,published_at:'2026-11-16T00:00:00Z'};
 assert.deepEqual(selectChannels([{...preview,draft:true},stable,preview]),{stable,preview});
 assert.equal(selectChannels([preview]).stable,null);
 assert.equal(selectChannels([preview,{...preview,published_at:'invalid'}]).preview,preview);
});
test('verified metadata produces actual asset URLs, hashes and exact build identity',()=>{const {release,content}=fixture();const m=manifest(release,content);assert.equal(m.commit,'a'.repeat(40));assert.equal(m.channel,'preview');assert.equal(m.files['leaguepingo.exe'].sha256,hash(content['leaguepingo.exe']));});
test('missing installers, modified metadata, wrong commits and foreign URLs fail closed',()=>{
 for(const mutate of [r=>r.assets.pop(),r=>r.target_commitish='b'.repeat(40),r=>r.assets[0].browser_download_url='https://example.com/evil',r=>r.assets.push(r.assets[0]),r=>r.draft=true]){const{release,content}=fixture();mutate(release);assert.throws(()=>manifest(release,content));}
 const{release,content}=fixture();content['BUILD-INFO.json']=Buffer.from('{}');assert.throws(()=>manifest(release,content),/checksum/);
});
test('deployment waits for release and PRs cannot deploy; permissions and actions remain restricted',()=>{
 const ci=fs.readFileSync(new URL('../../.github/workflows/ci.yml',import.meta.url),'utf8');
 const workflow=fs.readFileSync(new URL('../../.github/workflows/website.yml',import.meta.url),'utf8');
 assert.match(ci,/website:[\s\S]*needs: release/);assert.match(workflow,/needs: build/);assert.match(workflow,/github.event_name != 'pull_request'/);
 assert.equal((workflow.match(/contents: write/g)||[]).length,0);assert.match(workflow,/id-token: write/);assert.match(workflow,/npm --prefix website run sync-release/);
 for(const action of workflow.matchAll(/uses: (\S+)/g))assert.match(action[1],/^actions\/(checkout|setup-node|upload-pages-artifact|deploy-pages)@[a-f0-9]{40}$/);
});
