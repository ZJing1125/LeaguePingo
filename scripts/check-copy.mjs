// No new parser dependencies: use the locked frontend TypeScript/Vue toolchain.
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const require = createRequire(path.join(root, 'frontend/package.json'));
const { parse } = require('@vue/compiler-sfc');
const { baseParse } = require('@vue/compiler-dom');
const ts = require('typescript');
const chinese = /\p{Script=Han}/u;

export function checkComponentCopy(components) {
  for (const [file, source] of Object.entries(components)) {
    const fail = () => { throw new Error(`Move component copy to uiText: ${file}`); };
    function script(code) {
      const ast = ts.createSourceFile(file + '.ts', code, ts.ScriptTarget.Latest, true);
      function visit(node) {
        // Cooked values catch Unicode escapes too. Comments are not copy.
        if ((ts.isStringLiteralLike(node) || ts.isTemplateHead(node) || ts.isTemplateMiddle(node) || ts.isTemplateTail(node) || ts.isRegularExpressionLiteral(node)) && chinese.test(node.text)) fail();
        ts.forEachChild(node, visit);
      }
      visit(ast);
    }
    function template(node) {
      if (node.type === 2 && chinese.test(node.content)) fail();
      if (node.type === 5) script(node.content.content);
      for (const prop of node.props ?? []) {
        if (prop.type === 6 && prop.value && chinese.test(prop.value.content)) fail();
        if (prop.type === 7 && prop.exp) script(prop.exp.content);
      }
      for (const child of node.children ?? []) template(child);
    }
    const { descriptor, errors } = parse(source, { filename: file });
    if (errors.length) throw new Error(`Invalid Vue component: ${file}`);
    for (const block of [descriptor.script, descriptor.scriptSetup]) if (block) script(block.content);
    if (descriptor.template) template(baseParse(descriptor.template.content));
  }
  return { components: Object.keys(components).length };
}

function files(folder) {
  return fs.readdirSync(folder, { withFileTypes: true }).flatMap(entry => {
    const file = path.join(folder, entry.name);
    if (entry.isDirectory()) return files(file);
    return entry.isFile() && entry.name.endsWith('.vue') ? [file] : [];
  });
}

function run() {
  const components = Object.fromEntries(files(path.join(root, 'frontend/src')).map(file => [path.relative(root, file), fs.readFileSync(file, 'utf8')]));
  console.log(JSON.stringify({ ok: true, ...checkComponentCopy(components) }));
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { run(); }
  catch (error) { console.error(error.message); process.exitCode = 1; }
}
