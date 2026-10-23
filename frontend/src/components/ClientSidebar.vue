<script setup lang="ts">
import { computed } from 'vue';
import type { Snapshot } from '../../bindings/leaguepingo/internal/core/models';
import type { OverlayState } from '../../bindings/leaguepingo/models';
import BrandIcon from './BrandIcon.vue';
import AppIcon from './AppIcon.vue';
import QueueSwitch from './QueueSwitch.vue';
import SidebarPlayer from './SidebarPlayer.vue';
const props=defineProps<{snapshot:Snapshot|null;state:OverlayState|null;status:string;error:string}>();
defineEmits<{collapse:[value:boolean];queue:[value:number]}>();
const players=computed(()=>props.snapshot?.participants?.filter(p=>p.team==='ally') ?? []);
const picking=computed(()=>props.snapshot?.phase==='ChampSelect');
</script>
<template>
  <main v-if="state?.mode==='rail'" class="sidebar-rail desktop-app">
    <button aria-label="展开客户端侧栏" title="展开客户端侧栏" @click="$emit('collapse',false)"><BrandIcon/><span>{{ picking?'选人资料':'房间成员' }}</span><b>{{ players.length }}</b><AppIcon name="panel" :size="14"/></button>
  </main>
  <main v-else class="client-sidebar desktop-app" :data-side="state?.sidebarSide" aria-label="客户端玩家侧栏">
    <header class="sidebar-header"><BrandIcon/><strong>LeaguePingo</strong><button class="icon-button" aria-label="收起客户端侧栏" title="收起侧栏" @click="$emit('collapse',true)"><AppIcon name="minimise" :size="17"/></button></header>
    <div class="sidebar-heading"><div><h1>{{ picking?'我方阵容':'房间成员' }}</h1><small>{{ status.split(' · ')[0] }} · {{ players.length }} 人</small></div><QueueSwitch :value="snapshot?.scoutQueue ?? 420" compact @change="$emit('queue',$event)"/></div>
    <p v-if="error || state?.warning || snapshot?.rosterError" class="quality-warning" role="status">{{ error || state?.warning || snapshot?.rosterError }}</p>
    <section class="sidebar-players" aria-label="我方玩家资料">
      <SidebarPlayer v-for="p in players" :key="p.key" :player="p" :queue="snapshot?.scoutQueue ?? 420" :picking="picking"/>
      <p v-if="!players.length" class="sidebar-empty">{{ picking?'正在读取我方名单…':'等待房间成员…' }}</p>
    </section>
    <footer class="sidebar-footer"><span>{{ picking?'点击玩家展开完整统计':'进入选人后展示英雄表现' }}</span><small>{{ state?.holdEnabled && state?.holdAvailable ? `局内按住 ${state?.holdKey==='Backquote'?'~':state?.holdKey || '~'} 查看双方` : state?.shortcutAvailable ? `${state.shortcut} 显示局内快览` : '局内快览可在主程序调出' }}</small></footer>
  </main>
</template>
