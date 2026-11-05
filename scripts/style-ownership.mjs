import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import fs from 'node:fs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const require = createRequire(path.join(root, 'frontend/package.json'));
const { parse: parseSFC } = require('@vue/compiler-sfc');
const { parse: parseDOM } = require('@vue/compiler-dom');
const ts = require('typescript');
const tree = require('css-tree');

export const styleOwners = JSON.parse(fs.readFileSync(path.join(root, 'scripts/component-style-owners.json'), 'utf8'));
const utilities = new Set(styleOwners.sharedUtilities);
const scopes = new Set(styleOwners.publicScopes);

export function templateClasses(source, filename) {
  const classes = new Set();
  const { descriptor, errors } = parseSFC(source, { filename });
  if (errors.length) throw new Error(`Invalid Vue component: ${filename}`);
  const add = value => value.split(/\s+/).forEach(name => {
    if (/^[A-Za-z_][\w-]*$/.test(name)) classes.add(name);
  });
  const expression = value => {
    const ast = ts.createSourceFile('class.ts', `(${value})`, ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
    const visit = node => {
      if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node)) add(node.text);
      if ((ts.isPropertyAssignment(node) || ts.isShorthandPropertyAssignment(node)) && ts.isIdentifier(node.name)) add(node.name.text);
      ts.forEachChild(node, visit);
    };
    visit(ast);
  };
  const visit = node => {
    if (node.type === 1) {
      for (const prop of node.props) {
        if (prop.type === 6 && prop.value && /class$/i.test(prop.name)) add(prop.value.content);
        if (prop.type === 7 && prop.name === 'bind' && prop.arg?.content === 'class' && prop.exp) expression(prop.exp.content);
        if (prop.type === 6 && prop.name === 'name' && /transition/i.test(node.tag) && prop.value) {
          for (const suffix of ['enter-active', 'leave-active', 'enter-from', 'enter-to', 'leave-from', 'leave-to']) classes.add(`${prop.value.content}-${suffix}`);
        }
      }
    }
    for (const child of node.children || []) visit(child);
  };
  if (descriptor.template) visit(parseDOM(descriptor.template.content));
  return classes;
}

export function classUsage(components) {
  const uses = new Map();
  for (const [file, source] of Object.entries(components)) {
    for (const name of templateClasses(source, file)) {
      if (!uses.has(name)) uses.set(name, new Set());
      uses.get(name).add(file);
    }
  }
  return uses;
}

function publicScopeGroup(node) {
  if (node.type !== 'PseudoClassSelector' || !['is', 'where'].includes(node.name)) return false;
  const list = node.children?.first;
  if (list?.type !== 'SelectorList' || list.children.size < 2) return false;
  const names = [];
  for (const selector of list.children) {
    const items = [...selector.children];
    if (items.length !== 1 || items[0].type !== 'ClassSelector') return false;
    names.push(tree.ident.decode(items[0].name));
  }
  return names.includes('desktop-app') && names.every(name => scopes.has(name));
}

export function positiveClasses(selector) {
  const names = new Set();
  const visit = node => {
    if (node.type === 'ClassSelector') names.add(tree.ident.decode(node.name));
    // A negative/relational reference does not give the selected element an owner.
    if (node.type === 'PseudoClassSelector' && (['not', 'has'].includes(node.name) || publicScopeGroup(node))) return;
    if (node.children) for (const child of node.children) visit(child);
  };
  visit(tree.parse(selector, { context: 'selector' }));
  return names;
}

export function privateReferences(selector, uses) {
  const owners = new Map();
  for (const name of positiveClasses(selector)) {
    const files = uses.get(name);
    if (!utilities.has(name) && files?.size === 1) owners.set(name, [...files][0]);
  }
  return owners;
}

export function keyframeStep(rule) {
  return rule.parent.type === 'atrule' && /keyframes$/i.test(rule.parent.name);
}
