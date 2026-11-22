import {chromium} from 'playwright';
import assert from 'node:assert/strict';
import http from 'node:http';
import fs from 'node:fs';
import path from 'node:path';
const root=path.resolve(process.env.WEBSITE_DIST||'dist');
const server=http.createServer((req,res)=>{const relative=decodeURIComponent(new URL(req.url,'http://local').pathname).replace(/^\/LeaguePingo\//,'/');const file=path.resolve(root,'.'+(relative==='/'?'/index.html':relative));if(!file.startsWith(root+path.sep)){res.writeHead(403).end();return;}try{const ext=path.extname(file);res.setHeader('Content-Type',({'.html':'text/html','.js':'text/javascript','.css':'text/css','.webp':'image/webp','.png':'image/png','.json':'application/json'})[ext]||'application/octet-stream');res.end(fs.readFileSync(file));}catch{res.writeHead(404).end();}});
await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
const url=`http://127.0.0.1:${server.address().port}${process.env.WEBSITE_BASE||'/LeaguePingo/'}`;
const browser=await chromium.launch({executablePath:process.env.BROWSER_EXECUTABLE,headless:true});
try{
 const errors=[];
 for(const width of [1440,1280,768,390]){
  const page=await browser.newPage({viewport:{width,height:900},userAgent:'Mozilla/5.0 (Windows NT 10.0; Win64; x64)',reducedMotion:'reduce'});
  page.on('pageerror',e=>errors.push(e.message));
  await page.goto(url);await page.evaluate(()=>Promise.all([...document.images].filter(i=>!i.closest('[hidden]')).map(i=>i.decode())));
  const data=await(await page.request.get(url+'downloads.json')).json(),current=data.stable??data.preview;
  assert.equal(await page.locator('[data-download="installer"]').getAttribute('href'),current.files['leaguepingo-setup.exe'].url);
  assert.equal(await page.locator('[data-download="portable"]').getAttribute('href'),current.files['leaguepingo.exe'].url);
  assert.equal(await page.locator('.platform-note').isVisible(),false);
  assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'no horizontal page overflow');
  assert(await page.locator('[data-download="installer"]').evaluate(el=>el.getBoundingClientRect().bottom<innerHeight),'download on first screen');
  if(process.env.WEBSITE_SCREENSHOTS){fs.mkdirSync(process.env.WEBSITE_SCREENSHOTS,{recursive:true});await page.screenshot({path:path.join(process.env.WEBSITE_SCREENSHOTS,`website-${width}.png`),fullPage:true});}
  await page.getByRole('button',{name:'局内',exact:true}).click();await page.locator('#stage-ingame img').evaluate(i=>i.decode());assert(await page.locator('#stage-ingame').isVisible());
  await page.getByRole('button',{name:'局内',exact:true}).press('ArrowRight');assert(await page.locator('#stage-review').isVisible());assert.equal(await page.getByRole('button',{name:'复盘',exact:true}).getAttribute('aria-pressed'),'true');
  await page.getByRole('button',{name:'复盘',exact:true}).press('Home');assert(await page.locator('#stage-pregame').isVisible());
  await page.getByText('支持什么系统和服务器？',{exact:true}).click();assert(await page.getByText(/当前提供 Windows 64 位版本/).isVisible());
  assert(await page.evaluate(()=>[...document.images].filter(i=>!i.closest('[hidden]')).every(i=>i.complete&&i.naturalWidth>0)),'all displayed media loads');

  await page.close();
 }
 const noJS=await browser.newPage({javaScriptEnabled:false,viewport:{width:390,height:900}});await noJS.goto(url);assert(await noJS.locator('[data-download="installer"]').isVisible());assert(await noJS.locator('#stage-pregame').isVisible());assert.equal(await noJS.locator('.stage-controls').isVisible(),false);await noJS.close();
 const mac=await browser.newPage({userAgent:'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)'});await mac.goto(url);assert(await mac.locator('.platform-note').isVisible());await mac.close();
 assert.deepEqual(errors,[]);console.log('PASS: four responsive widths, real download manifest, keyboard stages, FAQ, media, Windows/macOS, no-script fallback and reduced motion');
}finally{await browser.close();await new Promise(resolve=>server.close(resolve));}
