// Synthetic post-game bridge. Never connects to a real client or starts a queue.
const messages = require('../../internal/apperrors/messages.json');
const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
const path=require('node:path');
const {chromium}=require('playwright');
const {mockWindowRuntime,normalizeServiceRequest}=require('./window-runtime.cjs');
(async()=>{
 const browser=await chromium.launch({headless:true,executablePath:process.env.BROWSER_EXECUTABLE});
 const releases=[];
 function barrier(){let release,entered;const gate=new Promise(resolve=>{release=resolve}),started=new Promise(resolve=>{entered=resolve});releases.push(release);return{gate,started,entered,release};}
 try {
 const context=await browser.newContext({viewport:{width:1280,height:800}});
 const binding=await fs.readFile(path.join(__dirname,'../bindings/leaguepingo/appservice.ts'),'utf8');
 const methods=Object.fromEntries([...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g)].map(m=>[m[2],m[1]]));
 let info={autoSkipHonor:false,version:'synthetic',commit:'auto-accept-test',requests:{},shortcut:'Ctrl+Shift+P',holdEnabled:true,holdKey:'Backquote',sidebarEnabled:true,previewEnabled:true,sidebarWidth:320,autoAcceptEnabled:false,autoAcceptDelay:1,autoReturnEnabled:false,autoQueueEnabled:false,autoQueueDelay:10};
 let state={skipHonor:false,revision:0,returnEnabled:false,queueEnabled:false,delaySeconds:10,status:'off',dueAt:'0001-01-01T00:00:00Z'};
 const snapshot={connected:true,connectionId:1,generation:1,phase:'Matchmaking',stage:'party',participants:[],updatedAt:new Date().toISOString()};
 const calls=[],errors=[],pages=[];let failSave=false,failInfo=false,infoBarrier=null,saveBarrier=null;
 async function open(url,width=1280){
 const page=await context.newPage();pages.push(page);await page.setViewportSize({width,height:800});page.on('pageerror',err=>errors.push(err.message));const runtime=mockWindowRuntime(page);
 await page.route('**/*',async route=>{
 const url=new URL(route.request().url());if(url.origin!=='http://127.0.0.1:43401')throw Error('unexpected external request');
 if(url.pathname==='/wails/custom.js')return route.fulfill({contentType:'text/javascript',body:''});
 if(url.pathname==='/wails/runtime'){
 const request=normalizeServiceRequest(route.request().postDataJSON());if(await runtime.handle(route,request))return;
 const method=methods[request.args?.methodID],args=request.args?.args??[];calls.push(method);let result=null;
 if(method==='Snapshot')result=snapshot;
 else if (method === 'GuideStatus') result={revision:0,status:'idle',query:{},base:null,matchup:null};
 else if (method === 'PostGameStatus') result=state;
 else if(method==='AutoAcceptStatus')result={revision:0,enabled:false,delaySeconds:1,status:'off',dueAt:'0001-01-01T00:00:00Z'};
 else if(method==='Info'){
 if(infoBarrier){const pending=infoBarrier;pending.entered();await pending.gate;}
 if(failInfo)return route.fulfill({status:500,json:{kind:'RuntimeError',message:'synthetic info failure',cause:{code:'DataUnavailable'}}});
 result=info;
 }
 else if(method==='OverlayStatus')result={revision:1,mode:'sidebar',visible:true,sidebarEnabled:true,sidebarWidth:320,holdEnabled:true};
 else if(method==='ConfigurePostGame'){
 if(saveBarrier){const pending=saveBarrier;pending.entered();await pending.gate;}
 if(failSave)return route.fulfill({status:400,json:{kind:'RuntimeError',message:'LCU HTTP 500 private-player#TEST',cause:{code:'SettingsWriteFailed'}}});
 info={...info,autoReturnEnabled:args[0],autoQueueEnabled:args[1],autoQueueDelay:args[2],autoSkipHonor:args[3]};state={...state,revision:state.revision+1,returnEnabled:args[0],queueEnabled:args[1],delaySeconds:args[2],skipHonor:args[3],status:args[0]||args[1]||args[3]?'armed':'off'};result=info;
 }else if(method==='CancelPostGame'){assert(['honor-countdown','return-countdown','queue-countdown'].includes(state.status));result=state={...state,revision:state.revision+1,status:'cancelled'};}
 else if(method!=='AuxiliaryReady')throw Error('unexpected service '+method);
 return route.fulfill({json:result});
 }
 const rel=url.pathname==='/'?'index.html':url.pathname.slice(1);assert(!rel.includes('..'));
 return route.fulfill({body:await fs.readFile(path.join(__dirname,'../dist',rel)),contentType:rel.endsWith('.js')?'text/javascript':rel.endsWith('.css')?'text/css':rel.endsWith('.png')?'image/png':'text/html'});
 });
 await page.goto('http://127.0.0.1:43401/'+url);return page;
 }
 async function emit(value=state){await Promise.all(pages.map(page=>page.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:post-game',data}),value)));}
 const page=await open('');
 await page.getByRole('button',{name:'游戏辅助',exact:true}).click();
 await page.locator('.automation-workspace').waitFor();
 const skip=page.getByRole('checkbox',{name:'自动跳过点赞',exact:true});
 const ret=page.getByRole('checkbox',{name:'赛后自动返回房间',exact:true}),queue=page.getByRole('checkbox',{name:'返回后自动排队',exact:true}),delay=page.getByRole('combobox',{name:'自动排队倒计时'});
 assert.equal(await ret.isChecked(),false);assert.equal(await queue.isChecked(),false);assert.equal(await delay.inputValue(),'10');assert.equal(await skip.isChecked(),false);
 await skip.check();await page.getByRole('button',{name:'保存自动化设置'}).click();await page.getByText('自动化设置已保存。',{exact:true}).waitFor();await emit();
 assert(info.autoSkipHonor && !info.autoReturnEnabled && !info.autoQueueEnabled,'skip-only option enabled return/queue');
 await page.locator('.automation-chip').getByText('自动 1 项',{exact:true}).waitFor();
 assert.equal(await page.locator('.post-game-notice').count(),0,'armed state must not render a banner');
 // Hold Info deliberately: rendering the workspace is not proof its settings
 // have arrived. Never use a sleep or read transient defaults as saved values.
 infoBarrier=barrier();
 await page.getByRole('button',{name:'首页',exact:true}).click();await page.getByRole('button',{name:'游戏辅助',exact:true}).click();
 await infoBarrier.started;
 const workspace=page.locator('.automation-workspace');
 assert.equal(await workspace.getAttribute('aria-busy'),'true','settings load must be observable');
 for(const control of [skip,ret,queue,delay])assert.equal(await control.isDisabled(),true,'unloaded settings were editable');
 assert.equal(calls.filter(c=>c==='ConfigurePostGame').length,1,'loading settings wrote automation');
 infoBarrier.release();infoBarrier=null;
 await page.locator('.automation-workspace[aria-busy="false"]').waitFor();
 assert.equal(await skip.isChecked(),true,'saved skip state not restored');
 // Failed initial loads keep controls unavailable; explicit local Info retry
 // restores the persisted values without any configure call.
 failInfo=true;
 await page.getByRole('button',{name:'首页',exact:true}).click();await page.getByRole('button',{name:'游戏辅助',exact:true}).click();
 await page.getByText(messages.DataUnavailable,{exact:true}).waitFor();
 assert.equal(await skip.isDisabled(),true);
 assert.equal(await page.getByRole('button',{name:'保存自动化设置',exact:true}).isDisabled(),true);
 failInfo=false;await page.getByRole('button',{name:'重新读取设置',exact:true}).click();
 await page.locator('.automation-workspace[aria-busy="false"]').waitFor();
 assert.equal(await skip.isChecked(),true,'Info retry lost the saved option');
 assert.equal(calls.filter(c=>c==='ConfigurePostGame').length,1,'Info retry wrote automation');
 await ret.check();await queue.check();await delay.selectOption('15');
 saveBarrier=barrier();await page.getByRole('button',{name:'保存自动化设置'}).click();await saveBarrier.started;
 for(const control of [skip,ret,queue,delay])assert.equal(await control.isDisabled(),true,'saving allowed drafts that the response could overwrite');
 saveBarrier.release();saveBarrier=null;
 await page.getByText('自动化设置已保存。',{exact:true}).waitFor();await emit();
 assert.equal(calls.filter(c=>c==='ConfigurePostGame').length,2);assert.equal(info.autoQueueDelay,15);
 failSave=true;await skip.uncheck();await delay.selectOption('20');await page.getByRole('button',{name:'保存自动化设置'}).click();
 await page.getByText(messages.SettingsWriteFailed,{exact:true}).waitFor();assert.equal(info.autoQueueDelay,15);assert.equal(info.autoSkipHonor,true,'failed save changed active skip');failSave=false;
 const side=await open('?view=companion',280);await side.locator('.client-sidebar').waitFor();
 for(const status of ['honor-countdown','return-countdown','queue-countdown']){
  state={...state,revision:state.revision+1,status,dueAt:new Date(Date.now()+10000).toISOString()};await emit();
  await side.locator('.post-game-notice').getByRole('button',{name:'取消本次',exact:true}).click();
  // The input click finishes before the asynchronous bridge response renders.
  await side.locator('.post-game-notice').waitFor({state:'hidden'});await emit();
  assert.equal(await side.locator('.post-game-notice').count(),0,'cancelled step must leave no banner');
 }
 assert.equal(calls.filter(c=>c==='CancelPostGame').length,3);
 await emit({...state,revision:state.revision+1,status:'queued'});assert.equal(await page.locator('.post-game-notice').count(),0,'terminal success stays off the workspace');
 state={...state,revision:state.revision+1,status:'continuing-postgame'};await emit();
 await side.getByText('正在完成赛后继续步骤…',{exact:true}).waitFor();
 state={...state,revision:state.revision+1,status:'honor-unconfirmed'};await emit();
 await side.getByText('未确认点赞页已结束，请在客户端检查',{exact:true}).waitFor();
 assert(await side.locator('.post-game-notice').evaluate(el=>el.scrollWidth<=el.clientWidth));
 for(const width of [880,1280]){await page.setViewportSize({width,height:800});assert(await page.locator('.automation-chip').evaluate(el=>el.scrollWidth<=el.clientWidth));}
 await page.getByRole('combobox',{name:'界面主题',exact:true}).selectOption('light');
 // The settings game tab no longer duplicates automation switches.
 await page.getByRole('button',{name:'设置与诊断',exact:true}).click();await page.getByRole('tab',{name:'游戏辅助'}).click();
 await page.getByRole('button',{name:'打开游戏辅助页',exact:true}).click();await page.locator('.automation-workspace').waitFor();
 await ret.uncheck();await page.getByText('手动返回房间后，才会开始排队倒计时。',{exact:true}).waitFor();
 await skip.uncheck();await page.getByRole('button',{name:'保存自动化设置'}).click();await page.getByText('自动化设置已保存。',{exact:true}).waitFor();await emit();
 assert.equal(info.autoSkipHonor,false);assert.equal(info.autoReturnEnabled,false);
 if(process.env.UI_SCREENSHOT_PREFIX){await page.locator('.automation-workspace').scrollIntoViewIfNeeded();await page.screenshot({path:process.env.UI_SCREENSHOT_PREFIX+'-automation.png'});}
 assert.deepEqual(errors,[]);console.log('PASS: automation opt-ins, delayed/failed Info reads and explicit retry, persisted switches, in-flight draft locking/save rollback, cross-window countdown cancellation, settings link and 880/280 layouts (synthetic bridge).');
 }finally{for(const release of releases)release();await browser.close();}
})().catch(err=>{console.error(err);process.exitCode=1});
