// Synthetic post-game bridge. Never connects to a real client or starts a queue.
const assert=require('node:assert/strict');
const fs=require('node:fs/promises');
const path=require('node:path');
const {chromium}=require('playwright');
const {mockWindowRuntime}=require('./window-runtime.cjs');
(async()=>{
 const browser=await chromium.launch({headless:true,executablePath:process.env.BROWSER_EXECUTABLE});
 try {
 const context=await browser.newContext({viewport:{width:1280,height:800}});
 const binding=await fs.readFile(path.join(__dirname,'../bindings/leaguepingo/appservice.ts'),'utf8');
 const methods=Object.fromEntries([...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g)].map(m=>[m[2],m[1]]));
 let info={version:'synthetic',commit:'auto-accept-test',requests:{},shortcut:'Ctrl+Shift+P',holdEnabled:true,holdKey:'Backquote',sidebarEnabled:true,previewEnabled:true,sidebarWidth:320,autoAcceptEnabled:false,autoAcceptDelay:1,autoReturnEnabled:false,autoQueueEnabled:false,autoQueueDelay:10};
 let state={revision:0,returnEnabled:false,queueEnabled:false,delaySeconds:10,status:'off',dueAt:'0001-01-01T00:00:00Z'};
 const snapshot={connected:true,connectionId:1,generation:1,phase:'Matchmaking',stage:'party',participants:[],updatedAt:new Date().toISOString()};
 const calls=[],errors=[],pages=[];let failSave=false;
 async function open(url,width=1280){
 const page=await context.newPage();pages.push(page);await page.setViewportSize({width,height:800});page.on('pageerror',err=>errors.push(err.message));const runtime=mockWindowRuntime(page);
 await page.route('**/*',async route=>{
 const url=new URL(route.request().url());if(url.origin!=='http://127.0.0.1:43401')throw Error('unexpected external request');
 if(url.pathname==='/wails/custom.js')return route.fulfill({contentType:'text/javascript',body:''});
 if(url.pathname==='/wails/runtime'){
 const request=route.request().postDataJSON();if(await runtime.handle(route,request))return;
 const method=methods[request.args?.methodID],args=request.args?.args??[];calls.push(method);let result=null;
 if(method==='Snapshot')result=snapshot;
 else if (method === 'GuideStatus') result={revision:0,status:'idle',query:{},base:null,matchup:null};
    else if (method === 'PostGameStatus') result=state;
    else if(method==='AutoAcceptStatus')result={revision:0,enabled:false,delaySeconds:1,status:'off',dueAt:'0001-01-01T00:00:00Z'};
 else if(method==='Info')result=info;
 else if(method==='OverlayStatus')result={revision:1,mode:'sidebar',visible:true,sidebarEnabled:true,sidebarWidth:320,holdEnabled:true};
 else if(method==='ConfigurePostGame'){
 if(failSave)return route.fulfill({status:400,json:{kind:'RuntimeError',message:'设置保存失败，原设置保留'}});
 info={...info,autoReturnEnabled:args[0],autoQueueEnabled:args[1],autoQueueDelay:args[2]};state={...state,revision:state.revision+1,returnEnabled:args[0],queueEnabled:args[1],delaySeconds:args[2],status:args[0]||args[1]?'armed':'off'};result=info;
 }else if(method==='CancelPostGame'){assert(['return-countdown','queue-countdown'].includes(state.status));result=state={...state,revision:state.revision+1,status:'cancelled'};}

 else if(method!=='AuxiliaryReady')throw Error('unexpected service '+method);
 return route.fulfill({json:result});
 }
 const rel=url.pathname==='/'?'index.html':url.pathname.slice(1);assert(!rel.includes('..'));
 return route.fulfill({body:await fs.readFile(path.join(__dirname,'../dist',rel)),contentType:rel.endsWith('.js')?'text/javascript':rel.endsWith('.css')?'text/css':rel.endsWith('.png')?'image/png':'text/html'});
 });
 await page.goto('http://127.0.0.1:43401/'+url);return page;
 }
 async function emit(value=state){await Promise.all(pages.map(page=>page.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:post-game',data}),value)));}
 const page=await open('');await page.getByRole('button',{name:'设置与诊断',exact:true}).click();await page.getByRole('tab',{name:'游戏辅助'}).click();
 const ret=page.getByRole('checkbox',{name:'赛后自动返回房间',exact:true}),queue=page.getByRole('checkbox',{name:'返回后自动排队',exact:true}),delay=page.getByRole('combobox',{name:'自动排队倒计时'});
 assert.equal(await ret.isChecked(),false);assert.equal(await queue.isChecked(),false);assert.equal(await delay.inputValue(),'10');
 await ret.check();await queue.check();await delay.selectOption('15');await page.getByRole('button',{name:'保存游戏辅助设置'}).click();await page.getByText('游戏辅助设置已保存。',{exact:true}).waitFor();await emit();
 assert.equal(calls.filter(c=>c==='ConfigurePostGame').length,1);assert.equal(info.autoQueueDelay,15);
 failSave=true;await delay.selectOption('20');await page.getByRole('button',{name:'保存游戏辅助设置'}).click();await page.getByText('设置保存失败，原设置保留',{exact:true}).waitFor();assert.equal(info.autoQueueDelay,15);failSave=false;
 await ret.scrollIntoViewIfNeeded();if(process.env.UI_SCREENSHOT_PREFIX)await page.screenshot({path:process.env.UI_SCREENSHOT_PREFIX+'-settings.png'});
 await page.getByRole('button',{name:'关闭设置',exact:true}).click();await page.getByText('赛后自动返回与排队已开启',{exact:true}).waitFor();
 const side=await open('?view=companion',280);await side.locator('.client-sidebar').waitFor();await side.locator('.post-game-notice').waitFor();
 for(const status of ['return-countdown','queue-countdown']){
  state={...state,revision:state.revision+1,status,dueAt:new Date(Date.now()+10000).toISOString()};await emit();
  await side.getByRole('button',{name:'取消本次',exact:true}).click();await emit();await page.getByText('本次赛后自动操作已取消',{exact:true}).waitFor();
 }
 await emit({...state,revision:state.revision-1,status:'queued'});assert.equal(await page.getByText('已开始排队',{exact:true}).count(),0);assert.equal(calls.filter(c=>c==='CancelPostGame').length,2);
 assert(await side.locator('.post-game-notice').evaluate(el=>el.scrollWidth<=el.clientWidth));
 for(const width of [880,1280]){await page.setViewportSize({width,height:800});assert(await page.locator('.post-game-notice').evaluate(el=>el.scrollWidth<=el.clientWidth));}
 await page.getByRole('combobox',{name:'界面主题',exact:true}).selectOption('light');
 await page.getByRole('button',{name:'设置与诊断',exact:true}).click();await page.getByRole('tab',{name:'游戏辅助'}).click();
 await ret.uncheck();await page.getByText('手动返回房间后，才会开始排队倒计时。',{exact:true}).waitFor();
 await page.getByRole('button',{name:'恢复默认'}).click();assert.equal(await ret.isChecked(),false);assert.equal(await queue.isChecked(),false);assert.equal(await delay.inputValue(),'10');assert.equal(info.autoQueueEnabled,true,'defaults must remain a draft');
 await page.getByRole('button',{name:'保存游戏辅助设置'}).click();await page.getByText('游戏辅助设置已保存。',{exact:true}).waitFor();await emit();assert.equal(info.autoQueueEnabled,false);
 if(process.env.UI_SCREENSHOT_PREFIX){await ret.scrollIntoViewIfNeeded();await page.screenshot({path:process.env.UI_SCREENSHOT_PREFIX+'-light.png'});}
 assert.deepEqual(errors,[]);console.log('PASS: post-game independent opt-in/defaults/save failures, manual-return guidance, cross-window return/queue countdown cancellation, stale revision and 880/280 light/dark layouts (synthetic bridge).');
 }finally{await browser.close();}
})().catch(err=>{console.error(err);process.exitCode=1});
