<script setup lang="ts">
import { computed, ref } from 'vue';
import type { Augment, State } from '../../bindings/leaguepingo/internal/guides/models';
import * as Service from '../../bindings/leaguepingo/appservice';
import { acceptGuide } from '../useGuide';
import { errorMessage } from '../presentation';
const props = defineProps<{ state: State; compact?: boolean; summary?: boolean }>();
const search = ref(''), rarity = ref(0), expanded = ref(false), busy = ref(false), error = ref('');
const augments = computed(() => props.state.base?.augments || []);
const selected = computed(() => props.state.candidates || []);
const compared = computed(() => selected.value.map(id => augments.value.find(a => a.id === id)).filter((a): a is Augment => !!a));
const matches = computed(() => augments.value.filter(a => (!rarity.value || a.rarity === rarity.value) && (!search.value.trim() || a.name.includes(search.value.trim()) || a.description.includes(search.value.trim()))));
const visible = computed(() => expanded.value || search.value.trim() ? matches.value : matches.value.slice(0, 6));
function quality(a: Augment) { return ({ 1: '银', 4: '金', 8: '棱彩' } as Record<number, string>)[a.rarity] || '品质未知'; }
function tier(a: Augment) { return a.tier === 0 ? 'OP级' : `${a.tier}级`; }
async function choose(id?: number) {
 if (busy.value) return;
 const ids = id == null ? [] : selected.value.includes(id) ? selected.value.filter(a => a !== id) : [...selected.value, id];
 if (ids.length > 3) return;
 busy.value = true; error.value = '';
 try { acceptGuide(await Service.GuideCandidates(props.state.scope, props.state.query.champion, ids)); }
 catch (e) { error.value = errorMessage(e); }
 finally { busy.value = false; }
}
</script>
<template>
 <section class="mayhem-augments" :class="{ compact, summary }" aria-label="海克斯强化符文">
  <div class="augment-heading"><h3>强化符文 <small>{{ augments.length }} 项</small></h3><span>OP.GG 梯度</span></div>
  <p class="augment-help">手动勾选本次出现的三个候选，比较当前英雄的强化表现。</p>
  <div class="augment-filters">
   <input v-model="search" type="search" aria-label="搜索强化符文" placeholder="搜索名称或效果" />
   <select v-model="rarity" aria-label="强化符文品质"><option :value="0">全部品质</option><option :value="1">银</option><option :value="4">金</option><option :value="8">棱彩</option></select>
  </div>
  <p v-if="error" class="augment-error" role="status">{{ error }}</p>
  <div v-if="compared.length" class="augment-comparison" aria-label="本次候选比较">
   <div class="compare-heading"><strong>本次候选 {{ compared.length }} / 3</strong><button class="secondary" :disabled="busy" @click="choose()">清空</button></div>
   <div class="compare-grid"><article v-for="a in compared" :key="a.id" class="compare-augment">
    <div><b>{{ a.name }}</b><span>{{ tier(a) }} · {{ quality(a) }}</span></div>
    <p>{{ a.description }}</p>
    <small>表现分 {{ a.performance == null ? '—' : a.performance.toFixed(1) }} · 热度分 {{ a.popularity == null ? '—' : a.popularity.toFixed(2) }}</small>
    <button class="secondary" :disabled="busy" :aria-label="`移除${a.name}`" @click="choose(a.id)">移除</button>
   </article></div>
  </div>
  <p v-if="state.base?.augmentsOmitted" class="augment-footnote">来源有 {{state.base.augmentsOmitted}} 项缺少名称或说明，未列入列表。</p>
  <div class="augment-list" aria-label="强化符文梯度列表">
   <label v-for="a in visible" :key="a.id" class="augment-row" :class="{ chosen: selected.includes(a.id) }">
    <input type="checkbox" :aria-label="`比较${a.name}`" :checked="selected.includes(a.id)" :disabled="busy || (!selected.includes(a.id) && selected.length >= 3)" @change="choose(a.id)" />
    <span class="augment-tier">{{ tier(a) }}</span>
    <span class="augment-body"><span><b>{{ a.name }}</b><small>{{ quality(a) }}</small></span><span class="augment-desc">{{ a.description }}</span></span>
   </label>
  </div>
  <p v-if="!matches.length" class="augment-help">没有匹配的强化符文。未收录不代表本局不会出现。</p>
  <button v-if="matches.length > 6 && !search.trim()" class="secondary augment-more" :aria-expanded="expanded" @click="expanded = !expanded">{{ expanded ? '收起列表' : `显示全部 ${matches.length} 项` }}</button>
  <p class="augment-footnote">表现分与热度分为来源指标。强化数据未提供胜率、样本量及版本；请结合本局选择。</p>
 </section>
</template>
<style scoped>
.mayhem-augments{width:100%;min-width:0;font-size:13px;color:var(--text)}
.augment-heading,.compare-heading{display:flex;align-items:center;justify-content:space-between;gap:8px}
.augment-heading h3{margin:0;font-size:14px}.augment-heading small,.augment-heading>span{font-size:11px;color:var(--muted);font-weight:400}
.augment-help,.augment-footnote{font-size:12px;line-height:1.65;color:var(--muted);margin:8px 0 12px}
.augment-filters{display:flex;gap:8px;margin-bottom:12px}.augment-filters input{flex:1;width:0;min-width:0}.augment-filters select{max-width:110px}.augment-filters input,.augment-filters select{font:inherit;font-size:12px;min-height:32px;border:1px solid var(--border);border-radius:5px;color:var(--text);background:var(--input-bg);padding:6px 8px}.augment-filters input::placeholder{color:var(--muted)}
.augment-list{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));column-gap:24px}.compact .augment-list{grid-template-columns:1fr}
.augment-row{display:flex;align-items:flex-start;gap:9px;padding:10px 0;border-top:1px solid var(--border);cursor:pointer;min-width:0}
.augment-row input{flex:none;width:14px;height:14px;margin:3px 0 0;accent-color:var(--accent)}
.augment-tier{flex:none;width:32px;padding-top:2px;font-size:12px;font-weight:700;color:var(--accent);font-variant-numeric:tabular-nums}
.augment-body{min-width:0;flex:1}.augment-body>span:first-child{display:flex;gap:8px;align-items:baseline;justify-content:space-between}.augment-body b{font-weight:600;overflow-wrap:anywhere}.augment-body small{font-size:11px;white-space:nowrap;color:var(--muted)}
.augment-desc{display:block;margin-top:5px;font-size:12px;line-height:1.65;color:var(--muted);white-space:pre-line;overflow-wrap:anywhere}
.chosen .augment-body b{color:var(--accent)}.augment-comparison{padding:12px;border:1px solid var(--border);margin:14px 0;border-radius:4px;background:var(--surface-2)}
.compare-heading{font-size:12px}.compare-heading button,.compare-augment button,.augment-more{font-size:12px;min-height:28px;border:1px solid var(--border);border-radius:5px;padding:4px 9px;color:var(--button-text);background:var(--button-bg);cursor:pointer}.mayhem-augments button:disabled{cursor:default;opacity:.7}.mayhem-augments :is(button,input,select):focus-visible{outline:2px solid var(--accent);outline-offset:2px}
.compare-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:14px;margin-top:12px}.compare-augment{min-width:0}.compare-augment>div{display:grid;gap:5px}.compare-augment>div span{color:var(--accent);font-size:12px}.compare-augment p{line-height:1.75;white-space:pre-line;overflow-wrap:anywhere;font-size:12px}.compare-augment small{display:block;color:var(--muted);font-size:11px;line-height:1.7;font-variant-numeric:tabular-nums}.compare-augment button{margin-top:8px}
.augment-more{margin-top:8px}.augment-footnote{margin:12px 0 0;font-size:11px}.augment-error{color:var(--enemy);font-size:12px}.compact .compare-grid{grid-template-columns:1fr}.compact .compare-augment+.compare-augment{border-top:1px solid var(--border);padding-top:10px}
@media(max-width:600px){.augment-list{grid-template-columns:1fr}.compare-grid{grid-template-columns:1fr}}
</style>
