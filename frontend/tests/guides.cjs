// Synthetic bridge only; no live client, OP.GG traffic, or player identifiers.
const assert=require('node:assert/strict');
const fs=require('node:fs/promises');const path=require('node:path');
const {chromium}=require('playwright');const {mockWindowRuntime}=require('./window-runtime.cjs');
const {makePlayer,now}=require('./analysis-fixture.cjs');
(async()=>{const browser=await chromium.launch({headless:true,executablePath:process.env.BROWSER_EXECUTABLE});try{
 const context=await browser.newContext({viewport:{width:1280,height:900}});
 const binding=await fs.readFile(path.join(__dirname,'../bindings/leaguepingo/appservice.ts'),'utf8');const methods=Object.fromEntries([...binding.matchAll(/export function (\w+)\([^]*?\$Call.ByID\((\d+)/g)].map(m=>[m[2],m[1]]));
 const own={...makePlayer(),key:'self',puuid:'self',championId:3,championState:'selected',position:'mid',team:'ally',premadeLabel:'A',premadeGroup:'ally-aabbcc',premadeSize:2,groupSource:'lcu'};
 const mate={...own,key:'mate',puuid:'mate'},enemy={...own,key:'enemy',puuid:'enemy',team:'enemy',championId:103,premadeGroup:'enemy-aabbcc'};
 const snapshot={connectionId:1,selfKey:'self',generation:1,gameId:42,queueId:420,scoutQueue:420,connected:true,phase:'ChampSelect',stage:'preparing',updatedAt:now,participants:[own,mate,enemy]};
 const query={mode:'ranked',champion:3,position:'mid',opponent:103,patch:'16.21'};
 const guide={query,version:'16.21',fetchedAt:now,sourceUrl:'https://op.gg/zh-cn/lol/champions/galio/build',builds:[{label:'核心装备',options:[{ids:[3152,3157,3089],names:['火箭腰带','中娅沙漏','灭世者的死亡之帽'],games:1200,winRate:52.5},{ids:[3152,3137],names:['火箭腰带','装备'],games:300,winRate:49}]}],runes:[8229,8226,8444],runeNames:['奥术彗星','法力流系带','复苏之风'],skills:['Q','W','E','Q','Q','R'],tips:[{label:'对手应对',text:'注意对手的控制技能，等待关键技能进入冷却。'},{label:'推荐打法',text:'先处理兵线，再选择支援时机。'}]};
 let state={revision:1,scope:'1:self:42',status:'ready',message:'',query,modeLabel:'单双排',reference:false,positionExplicit:true,base:guide,matchup:guide,stale:false,versionMismatch:false};
 const calls=[],errors=[],pages=[];
 async function open(view,width){const page=await context.newPage();pages.push(page);await page.setViewportSize({width,height:900});page.on('pageerror',e=>errors.push(e.message));const runtime=mockWindowRuntime(page);
 await page.route('**/*',async route=>{const url=new URL(route.request().url());assert.equal(url.origin,'http://127.0.0.1:43411');
 if(url.pathname==='/wails/custom.js')return route.fulfill({contentType:'text/javascript',body:''});
 if(url.pathname==='/wails/runtime'){const r=route.request().postDataJSON();if(await runtime.handle(route,r))return;const name=methods[r.args?.methodID],args=r.args?.args||[];calls.push({name,args});let result=null;
 if(name==='Snapshot')result=snapshot;
 else if(name==='GuideStatus')result=state;
 else if(name==='GuideCandidates'){assert.equal(args[0],state.scope);assert.equal(args[1],state.query.champion);state={...state,revision:state.revision+1,candidates:args[2]};result=state;for(const p of pages)await p.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:guide',data}),state);}
 else if(name==='SelectGuide'){state={...state,revision:state.revision+1,status:'loading',query:{...state.query,position:args[0],opponent:args[1]}};result=state;}
 else if(name==='Info')result={version:'synthetic',commit:'guides-test',requests:{},shortcut:'Ctrl+Shift+P',holdEnabled:true,holdKey:'Backquote',sidebarEnabled:true,sidebarWidth:320};
 else if(name==='OverlayStatus')result={revision:1,visible:true,mode:view.includes('overlay')?'manual':'sidebar',sidebarEnabled:true,sidebarWidth:320};
 else if(name==='AutoAcceptStatus')result={revision:0,enabled:false,status:'off'};
 else if(name==='PostGameStatus')result={revision:0,returnEnabled:false,queueEnabled:false,status:'off'};
 else if(['CurrentPlayer','EnsureProfile'].includes(name))result=own;
 else if(name==='HistoryPage'){assert.equal(args[2],20,'guide fixture started an unexpected deep scan');result={matches:[],offset:20,nextOffset:20,hasMore:false,meta:{returned:0,parsed:0,fetchedAt:now}};}
 else if(name!=='AuxiliaryReady')throw Error('unexpected guide bridge '+name);
 return route.fulfill({json:result});}
 const rel=url.pathname==='/'?'index.html':url.pathname.slice(1);assert(!rel.includes('..'));return route.fulfill({body:await fs.readFile(path.join(__dirname,'../dist',rel)),contentType:rel.endsWith('.js')?'text/javascript':rel.endsWith('.css')?'text/css':rel.endsWith('.png')?'image/png':rel.endsWith('.jpg')?'image/jpeg':'text/html'});
 });await page.goto('http://127.0.0.1:43411/'+view);return page;}
 const main=await open('',1280);await main.getByRole('button',{name:'对局室',exact:true}).click();await main.locator('.guide-panel .guide-build-row').first().waitFor();
 await main.getByLabel('英雄攻略与模式适配：实验性，国服实机验证未完成',{exact:true}).waitFor();
 await main.getByLabel('双方同组标识：同组实验性，国服实机验证未完成',{exact:true}).first().waitFor();
 assert.equal(await main.locator('.premade-badge[data-group="ally-aabbcc"]').count(),2);await main.getByRole('button',{name:'展开完整攻略',exact:true}).click();assert.equal(await main.locator('.guide-panel').getByText('52.5%',{exact:true}).count(),1);
 const side=await open('?view=companion',280);await side.getByRole('button',{name:'本局攻略',exact:true}).click();await side.locator('.guide-panel .guide-build-row').first().waitFor();
 await side.getByLabel('双方同组标识：同组实验性，国服实机验证未完成',{exact:true}).waitFor();
 assert(await side.locator('.sidebar-heading').evaluate(e=>e.scrollWidth<=e.clientWidth),'group label overflow at 280px');
 await side.getByLabel('英雄攻略与模式适配：实验性，国服实机验证未完成',{exact:true}).waitFor();
 assert(await side.locator('.guide-panel').evaluate(e=>e.scrollWidth<=e.clientWidth));await side.getByRole('button',{name:'查看备选方案',exact:true}).click();await side.getByText('49.0%',{exact:true}).waitFor();
 const overlay=await open('?view=overlay',1100);await overlay.setViewportSize({width:1100,height:640});await overlay.getByRole('button',{name:'本局攻略',exact:true}).click();await overlay.locator('.guide-panel').waitFor();assert(await overlay.locator('.guide-switch').evaluate(e=>e.scrollWidth<=e.clientWidth));if(process.env.UI_SCREENSHOT_PREFIX)await overlay.screenshot({path:process.env.UI_SCREENSHOT_PREFIX+'-overlay.png'});assert.equal(await overlay.locator('.overlay-player').count(),0);await overlay.getByRole('button',{name:'双方资料',exact:true}).click();assert.equal(await overlay.locator('.guide-panel').count(),0);assert.equal(await overlay.locator('.overlay-player').count(),3);
 const before=calls.filter(c=>c.name==='SelectGuide').length;await side.getByRole('button',{name:'玩家资料',exact:true}).click();await side.getByRole('button',{name:'本局攻略',exact:true}).click();assert.equal(calls.filter(c=>c.name==='SelectGuide').length,before);
 await main.getByRole('combobox',{name:'攻略分路'}).selectOption('support');await main.locator('.guide-loading').waitFor();assert.equal(calls.filter(c=>c.name==='SelectGuide').length,1);
 state={...state,revision:state.revision+1,status:'partial',message:'OP.GG 连接失败或超时',stale:true};for(const p of pages)await p.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:guide',data}),state);
 await side.getByText('OP.GG 连接失败或超时',{exact:true}).waitFor();assert.equal(await side.locator('.guide-build-row').count(),2,'cached builds retained');
 for(const p of pages)await p.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:guide',data}),{...state,revision:0,status:'unsupported',base:null,matchup:null});assert.equal(await side.locator('.guide-build-row').count(),2,'stale events ignored');
 for(const width of [880,1280]){await main.setViewportSize({width,height:900});assert(await main.locator('.guide-panel').evaluate(e=>e.scrollWidth<=e.clientWidth));}
 await main.getByRole('button',{name:'收起完整攻略',exact:true}).click();
 if(process.env.UI_SCREENSHOT_PREFIX){await main.screenshot({path:process.env.UI_SCREENSHOT_PREFIX+'-main-dark.png'});await side.screenshot({path:process.env.UI_SCREENSHOT_PREFIX+'-sidebar-dark.png'});}
 await main.getByRole('combobox',{name:'界面主题',exact:true}).selectOption('light');if(process.env.UI_SCREENSHOT_PREFIX)await main.screenshot({path:process.env.UI_SCREENSHOT_PREFIX+'-main-light.png'});
 state={...state,revision:state.revision+1,status:'unsupported',message:'当前模式暂无已核验攻略',base:null,matchup:null};for(const p of pages)await p.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:guide',data}),state);await side.getByText('当前模式暂无已核验攻略',{exact:true}).waitFor();assert.equal(await side.locator('.guide-build-row').count(),0);

 const augments=Array.from({length:8},(_,i)=>({id:1000+i,name:`强化${i+1}`,description:i===0?'对手控制较多时提供护盾，保持生存。':'提升技能与装备的配合效果。',tier:i%6,rarity:[1,4,8][i%3],performance:i===0?0:80+i,popularity:2+i/10}));
 snapshot.queueId=2400;
 state={...state,revision:state.revision+1,status:'ready',message:'',query:{mode:'aram-mayhem',champion:3,position:'mid',opponent:0,patch:''},modeLabel:'海克斯大乱斗',base:{...guide,augments,runes:[],skills:['Q','W','E'],sourceUrl:'https://op.gg/zh-cn/lol/modes/aram-mayhem/galio/build'},matchup:null,candidates:[],stale:false};
 for(const p of pages)await p.evaluate(({snapshot,state})=>{window._wails.dispatchWailsEvent({name:'leaguepingo:snapshot',data:snapshot});window._wails.dispatchWailsEvent({name:'leaguepingo:guide',data:state});},{snapshot,state});
 await side.getByRole('checkbox',{name:'比较强化1',exact:true}).waitFor();
 await side.getByLabel('英雄攻略与模式适配：实验性，国服实机验证未完成',{exact:true}).waitFor();
 assert.equal(calls.filter(c=>c.name==='SelectGuide').length,1,'experimental labels must not fetch guides');
 assert.equal(await side.getByRole('combobox',{name:'攻略分路'}).count(),0,'Mayhem must not require lanes');
 const guideCalls=calls.filter(c=>c.name==='SelectGuide').length;
 await side.getByRole('checkbox',{name:'比较强化1',exact:true}).check();
 await side.getByRole('checkbox',{name:'比较强化2',exact:true}).check();
 await side.getByRole('checkbox',{name:'比较强化3',exact:true}).check();
 await overlay.getByRole('button',{name:'本局攻略',exact:true}).click();
 await overlay.locator('.compare-augment').nth(2).waitFor();
 assert.equal(await side.getByRole('checkbox',{name:'比较强化4',exact:true}).isDisabled(),true);
 assert.equal(await overlay.locator('.compare-augment').count(),3,'choices shared across windows');
 assert.equal(await overlay.locator('.guide-panel').evaluate(e=>getComputedStyle(e).backgroundColor),'rgb(26, 36, 39)','overlay must keep dark contrast in light theme');
 await side.getByRole('searchbox',{name:'搜索强化符文'}).fill('强化8');assert.equal(await side.locator('.augment-row').count(),1);
 await side.getByRole('searchbox',{name:'搜索强化符文'}).fill('');
 await side.getByRole('combobox',{name:'强化符文品质'}).selectOption('8');assert.equal(await side.locator('.augment-row').count(),2);
 assert.equal(calls.filter(c=>c.name==='SelectGuide').length,guideCalls,'search/quality/compare must not fetch guides');
 for(const p of [main,side,overlay])assert(await p.locator('.mayhem-augments').evaluate(e=>e.scrollWidth<=e.clientWidth),'Mayhem overflow');
 assert(await side.locator('.compare-augment').first().innerText().then(t=>t.includes('表现分 0.0')),'known zero preserved');
 if(process.env.UI_SCREENSHOT_PREFIX){await main.screenshot({path:process.env.UI_SCREENSHOT_PREFIX+'-mayhem-main.png'});await side.screenshot({path:process.env.UI_SCREENSHOT_PREFIX+'-mayhem-sidebar.png'});await overlay.screenshot({path:process.env.UI_SCREENSHOT_PREFIX+'-mayhem-overlay.png'});}
 await side.getByRole('button',{name:'清空',exact:true}).click();await main.locator('.compare-augment').waitFor({state:'detached'});
 state={...state,revision:state.revision+1,status:'idle',query:{...state.query,champion:99},base:null,candidates:[]};for(const p of pages)await p.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:guide',data}),state);assert.equal(await side.locator('.augment-row').count(),0,'old champion augments cleared');

 state={...state,revision:state.revision+1,status:'ready',query:{...query,mode:'arena',opponent:0},modeLabel:'斗魂竞技场',base:{...guide,query:{...query,mode:'arena',opponent:0},augments:augments.map((a,i)=>({...a,winRate:i===0?0:52.5,games:1234,performance:null,popularity:null})),runes:[],builds:[{label:'核心装备',options:[{ids:[999999],names:['棱彩装备名称'],games:0,winRate:null}]}],tips:[{label:'虚空之女',text:'搭档组合胜率 51.59% · 1223 场'}]},matchup:null,candidates:[]};
 for(const p of pages)await p.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:guide',data}),state);
 await side.getByRole('checkbox',{name:'比较强化1',exact:true}).check();
 assert((await side.locator('.mayhem-augments').innerText()).includes('胜率 0.00%'),'Arena known zero win rate lost');
 assert(!(await side.locator('.mayhem-augments').innerText()).includes('OP级'),'Arena must not invent Mayhem tiers');
 assert(!(await side.locator('.compare-augment').innerText()).includes('表现分'),'Arena scores must not be fabricated');
 await side.getByText('棱彩装备名称',{exact:true}).waitFor();
 if(process.env.UI_SCREENSHOT_PREFIX)await side.screenshot({path:process.env.UI_SCREENSHOT_PREFIX+'-arena-sidebar.png'});
 for(const mode of ['urf','nexus-blitz']){
  state={...state,revision:state.revision+1,query:{...state.query,mode},modeLabel:mode==='urf'?'无限火力':'极限闪击',base:{...guide,augments:[],version:mode==='urf'?'16.04':'13.23',referenceNote:'轮换模式资料可能来自较早版本，请结合页面版本和当前规则使用。'},candidates:[]};
  for(const p of pages)await p.evaluate(data=>window._wails.dispatchWailsEvent({name:'leaguepingo:guide',data}),state);
  await side.getByText(state.base.referenceNote,{exact:true}).waitFor();
  assert.equal(await side.getByRole('combobox',{name:'攻略分路'}).count(),0,'rotating mode falsely requires a lane');
  assert.equal(await side.locator('.mayhem-augments').count(),0,'stale augment mode leaked');
 }
 assert.equal(calls.filter(c=>c.name==='GuideStatus').length,pages.length,'one memory read per window');assert.deepEqual(errors,[]);console.log('PASS: Arena real stats/rotating version references/mode isolation; Mayhem candidates/search/quality/shared-selection/dark-overlay/no-fetch; guide/cache/failure/mode/manual-position/cross-window/stale-event and group badges; 880/280 layouts, light/dark (synthetic).');
 }finally{await browser.close();}})().catch(e=>{console.error(e);process.exitCode=1});
