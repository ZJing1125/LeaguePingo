import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkComponentCopy } from './check-copy.mjs';

function component(body) { return { 'Root.vue': body }; }
test('central references, dynamic data and comments are allowed in roots and children', () => {
  const source = '<script setup>/* 中文注释 */ const data = profile.name;</script><template><!-- 中文注释 --><button :title="uiText.common.title">{{ uiText.common.count(data) }}</button></template><style scoped>/* 样式说明 */ div{color:red}</style>';
  assert.deepEqual(checkComponentCopy(component(source)), { components: 1 });
});
test('static text and accessibility attributes cannot bypass the catalog', () => {
  for (const body of ['<p>战绩</p>', '<button aria-label="关闭"/>', '<input placeholder="搜索"/>', '<p title="&#x6218;&#x7ee9;"/>']) {
    assert.throws(() => checkComponentCopy(component('<template>' + body + '</template>')), /Move component copy.*Root.vue/);
  }
});
test('script setup, ordinary scripts, Unicode escapes and nested templates are checked', () => {
  for (const code of ["const name = '战绩';", 'const name = "\\u6218\\u7ee9";', 'const name = `查看${count}场`;', "const name = `${ready ? '就绪' : value}`;", 'const name = /胜利/;']) {
    for (const tag of ['script', 'script setup']) assert.throws(() => checkComponentCopy(component(`<${tag}>${code}</script><template><div/></template>`)), /Move component copy/);
  }
});
test('directive values and interpolations are checked without treating comment text as copy', () => {
  for (const body of ['<p>{{ ready ? "就绪" : value }}</p>', '<button :title="`查看${count}场`"/>', '<button @click="notify(\'完成\')"/>', '<div v-for="item in [\'玩家\']"/>']) {
    assert.throws(() => checkComponentCopy(component('<template>' + body + '</template>')), /Move component copy/);
  }
});
test('invalid components fail instead of silently skipping copy checks', () => {
  assert.throws(() => checkComponentCopy(component('<template><div></template>')), /Invalid Vue component/);
});
