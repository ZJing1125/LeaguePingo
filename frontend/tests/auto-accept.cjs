// Synthetic desktop bridge; no real client, accounts, or automatic acceptance.
const messages = require('../../internal/apperrors/messages.json');
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
 let info={version:'synthetic',commit:'auto-accept-test',requests:{},shortcut:'Ctrl+Shift+P',holdEnabled:true,holdKey:'Backquote',sidebarEnabled:true,previewEnabled:true,sidebarWidth:320,autoAcceptEnabled:false,autoAcceptDelay:1,autoSkipHonor:false,autoReturnEnabled:false,autoQueueEnabled:false,autoQueueDelay:10};
 let state={revision:0,enabled:false,delaySeconds:1,status:'off',dueAt:'0001-01-01T00:00:00Z'};
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
 else if (method === 'PostGameStatus') result={revision:0,returnEnabled:false,queueEnabled:false,delaySeconds:10,status:'off',dueAt:'0001-01-01T00:00:00Z'};
 else if(method==='AutoAcceptStatus')result=state;
 else if(method==='Info')result=info;
 else if(method==='OverlayStatus')result={revision:1,mode:'sidebar',visible:true,sidebarEnabled:true,sidebarWidth:320,holdEnabled:true};
 else if(method==='ConfigureAutoAccept'){
 if(failSave)return route.fulfill({status:400,json:{kind:'RuntimeError',message:'LCU HTTP 500 private-player#TEST',cause:{code:'SettingsWriteFailed'}}});
 info={...info,autoAcceptEnabled:args[0],autoAcceptDelay:args[1]};state={...state,revision:state.revision+1,enabled:args[0],delaySeconds:args[1],status:args[0]?'armed':'off'};result=info;
 }else if(method==='SkipAutoAccept'){assert.equal(state.status,'countdown');result=state={...state,revision:state.revision+1,status:'skipped'};}
 else if(method!=='AuxiliaryReady')throw Error('unexpected service '+method);
 return route.fulfill({json:result});
 }
 const rel=url.pathname==='/'?'index.html':url.pathname.slice(1);assert(!rel.includes('..'));
 return route.fulfill({body:await fs.readFile(path.join(__dirname,'../dist',rel)),contentType:rel.endsWith('.js')?'text/javascript':rel.endsWith('.css')?'text/css':rel.endsWith('.png')?'image/png':'text/html'});
 });
 await page.goto('http://127.0.0.1:43401/'+url);return page;
 }
 async function emit(value=state){await Promise.all(pages.map(page=>page.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:auto-accept',data}),value)));}
 const page=await open('');
 await page.getByRole('button',{name:'游戏辅助',exact:true}).click();
 await page.locator('.automation-workspace').waitFor();
 const toggle=page.getByRole('checkbox',{name:'启用自动接受对局',exact:true}),delay=page.getByRole('combobox',{name:'自动接受延迟'});
 assert.equal(await toggle.isChecked(),false);assert.equal(await delay.inputValue(),'1');
 assert.equal(await page.getByText('国服实机验证未完成').count(),0,'validation note removed from the page');
 assert.equal(calls.filter(c=>c.startsWith('Configure')).length,0,'viewing automation must not enable it');
 await toggle.check();await delay.selectOption('3');await page.getByRole('button',{name:'保存自动化设置'}).click();
 await page.getByText('自动化设置已保存。',{exact:true}).waitFor();await emit();
 assert.equal(calls.filter(c=>c==='ConfigureAutoAccept').length,1);assert.equal(info.autoAcceptDelay,3);
 // Armed is shown once, as a title-bar chip, never as a static banner.
 await page.locator('.automation-chip').getByText('自动 1 项',{exact:true}).waitFor();
 assert.equal(await page.locator('.auto-accept-notice').count(),0,'armed state must not render a banner');
 failSave=true;await delay.selectOption('5');await page.getByRole('button',{name:'保存自动化设置'}).click();
 await page.getByText(messages.SettingsWriteFailed,{exact:true}).waitFor();assert.equal(info.autoAcceptDelay,3);failSave=false;
 const side=await open('?view=companion',280);await side.locator('.client-sidebar').waitFor();
 state={...state,revision:state.revision+1,status:'countdown',dueAt:new Date(Date.now()+5000).toISOString()};await emit();
 await page.locator('.auto-accept-notice').getByText(/秒后自动接受/).waitFor();
 await side.locator('.auto-accept-notice').getByRole('button',{name:'跳过本次',exact:true}).click();
 assert.equal(calls.filter(c=>c==='SkipAutoAccept').length,1);
 await emit();await page.waitForTimeout(50);
 assert.equal(await page.locator('.auto-accept-notice').count(),0,'finished skip must not leave a banner');
 await emit({...state,revision:state.revision-1,status:'accepted'});
 assert.equal(await page.locator('.auto-accept-notice').count(),0);
 assert(await side.locator('.client-sidebar').evaluate(el=>el.scrollWidth<=el.clientWidth));
 for(const width of [880,1280]){await page.setViewportSize({width,height:800});assert(await page.locator('.automation-chip').evaluate(el=>el.scrollWidth<=el.clientWidth));}
 await page.getByRole('combobox',{name:'界面主题',exact:true}).selectOption('light');
 // Settings now links to the automation page instead of duplicating its switches.
 await page.getByRole('button',{name:'设置与诊断',exact:true}).click();await page.getByRole('tab',{name:'游戏辅助'}).click();
 await page.getByRole('button',{name:'打开游戏辅助页',exact:true}).click();
 await page.locator('.automation-workspace').waitFor();
 assert.equal(await page.getByRole('checkbox',{name:'启用自动接受对局',exact:true}).count(),1,'automation switches live on one page only');
 await toggle.uncheck();await page.getByRole('button',{name:'保存自动化设置'}).click();await page.getByText('自动化设置已保存。',{exact:true}).waitFor();await emit();assert.equal(info.autoAcceptEnabled,false);
 await page.locator('.automation-chip').waitFor({state:'detached'});
 if(process.env.UI_SCREENSHOT_PREFIX)await page.screenshot({path:process.env.UI_SCREENSHOT_PREFIX+'-automation.png'});
 assert.deepEqual(errors,[]);console.log('PASS: automation page opt-in, chip instead of banners, failure rollback/default draft, cross-window countdown/skip, settings link and 880/280 geometry (synthetic bridge).');
 }finally{await browser.close();}
})().catch(err=>{console.error(err);process.exitCode=1});
