// Optional local evidence only; callers provide synthetic bridges, never accounts.
const fs=require('node:fs/promises');
async function saveUIArtifact(page,prefix,name,freeze=false){
 if(!prefix)return;
 await page.screenshot({path:`${prefix}-${name}.png`});
 if(!freeze)return;
 const html=await page.evaluate(()=>{
  const css=[...document.styleSheets].map(sheet=>{try{return [...sheet.cssRules].map(rule=>rule.cssText).join('\n');}catch{return '';}}).join('\n');
  const root=document.documentElement.cloneNode(true);
  root.querySelectorAll('script,link[rel="stylesheet"],link[rel="modulepreload"]').forEach(el=>el.remove());
  const style=document.createElement('style');style.textContent=css;root.querySelector('head').append(style);
  return '<!doctype html>'+root.outerHTML;
 });
 await fs.writeFile(`${prefix}-${name}.html`,html.replace(/https?:\/\/127\.0\.0\.1:\d+/g,'').replaceAll('/assets/','/frontend/dist/assets/').replaceAll('/brand/','/frontend/public/brand/'));
}
module.exports={saveUIArtifact};
