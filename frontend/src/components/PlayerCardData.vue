<script setup lang="ts">
import { computed } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import { percent, decimal, date } from "../presentation";
import { rankPresentation } from "../rankPresentation";
import { championName } from "../gameAssets";
import { scoutStats, roleNames } from "../scoutStats";
import GameIcon from "./GameIcon.vue";
import AppIcon from "./AppIcon.vue";
import RankEmblem from "./RankEmblem.vue";
import ScoutRing from "./ScoutRing.vue";
const props = defineProps<{ player: Participant; frozen?: boolean; party?: boolean; queue?: number; captured?: boolean }>();
const data = computed(() => scoutStats(props.player, props.queue ?? 420));
const rank = computed(() => rankPresentation(props.player, props.queue === 440 ? "RANKED_FLEX_SR" : "RANKED_SOLO_5x5"));
const season = computed(() => rank.value.record);
const hero = computed(() => props.player.championId ? championName(props.player.championId).split("·").at(-1) : props.party || props.player.championState === "unselected" ? "尚未选择英雄" : "英雄待确认");
const heroScope = computed(() => props.player.championState === "intent" ? "预选英雄" : props.player.championId ? data.value.report?.season ? data.value.report.complete ? '本季排位' : '本季已读' : '英雄表现'  : props.party ? "房间成员" : "等待选择");
const heroStatus = computed(() => {
  if (!props.player.championId) return props.party || props.player.championState === "unselected" ? "等待英雄选择" : "等待英雄信息";
  const report = data.value.report;
  if (!report || report.state === "loading") return props.captured ? "本局未完整保留统计" : "英雄统计准备中";
  if (!report.season) return "赛季范围待确认";
  if (data.value.champion.games) return "KDA 暂缺";
  return report.complete ? "本季暂无记录" : "本季记录未完整";
});
const range = computed(() => {
  const report = data.value.report;
  if (!report) return props.captured ? "统计未保留完整" : "统计准备中";
  if (report.retained) return "上次统计";
  if (report.state === "loading") return "统计准备中";
  if (report.complete30) return "近30天";
  return report.read ? `30天已读 ${report.read} 场` : "30天待读取";
});
const roleCaption = computed(() => ({ '位置未记录': '暂无记录', '位置记录较少': '记录较少', '位置较分散': '位置分散' }[data.value.role.label] || data.value.role.label));
const shortRank = computed(() => rank.value.rank ? rank.value.label : rank.value.label.includes('失败') ? '查询失败' : rank.value.label.includes('超时') ? '查询超时' : '段位待确认');
const roleTitle = computed(() => `近30天 · ${data.value.queueLabel}；有效位置 ${data.value.role.known}/${data.value.overall.games} 场。${data.value.role.positions.map(p => `${roleNames[p.position]} ${p.games} 场`).join('，')}；不代表本局分路。`);
function periodCaption(p: ReturnType<typeof scoutStats>["periods"][number]) {
  if (!data.value.report || !p.games && data.value.report.state === "loading") return props.captured ? "未保留" : "准备中";
  return p.outcomeGames ? `有效 ${p.outcomeGames} 场` : p.games ? `已读 ${p.games} 场` : p.complete ? '0 场' : '待读取';
}
function periodDetail(p: ReturnType<typeof scoutStats>["periods"][number]) {
  if (!p.games) return p.complete ? "无排位记录" : data.value.report?.state === "loading" || !data.value.report ? "—" : "范围未完整";
  return p.outcomeGames ? `${p.wins}胜 / ${p.losses}负` : "胜负未记录";
}
</script>
<template>
  <span class="card-details scout-data">
    <span class="scout-headline">
      <span class="scout-champion">
        <span class="card-pick"><GameIcon v-if="player.championId" :id="player.championId" :size="32" /><AppIcon v-else name="match" :size="28" /><span><strong :title="hero">{{ hero }}</strong><small :title="heroScope">{{ heroScope }}</small></span></span>
        <span v-if="data.champion.average" class="scout-kda" :title="`当前赛季 · ${data.queueLabel} · 本英雄 ${data.champion.kdaGames} 场有效记录的平均 K / D / A`"><b>{{ data.champion.average[0]!.toFixed(1) }}</b> / <b>{{ data.champion.average[1]!.toFixed(1) }}</b> / <b>{{ data.champion.average[2]!.toFixed(1) }}</b></span>
        <span v-else class="scout-no-kda" :title="heroStatus">{{ heroStatus }}</span>
        <small class="scout-hero-rate" :class="{ 'no-sample': !data.champion.outcomeGames }">{{ data.champion.outcomeGames ? percent(data.champion.winRate) : data.report?.season ? `本季${data.queueLabel}` : data.queueLabel }}<span v-if="data.champion.outcomeGames"> · {{ data.champion.wins }}胜 / {{ data.champion.losses }}负</span></small>
      </span>
      <span class="scout-rank" :title="`${data.queueLabel} · ${rank.title}`">
        <span class="scout-rank-heading"><RankEmblem :tier="player.identityState === 'known' ? rank.rank?.tier : undefined" /><small class="scout-queue-label">{{ data.queueLabel }}</small></span>
        <span class="rank-line"><b>{{ shortRank }}</b><small v-if="rank.retained">旧</small></span>
        <small class="scout-season" :title="season?.title"><span v-if="season">{{ season.scope }} {{ percent(season.rate) }}</span><span>{{ season ? `${season.wins}胜 / ${season.losses}负` : '胜负待确认' }}</span></small>
      </span>
    </span>
    <template v-if="player.identityState === 'known'">
      <span class="overview-metrics scout-rings" :title="`${data.queueLabel} · 独立时间范围统计，获取于 ${date(data.report?.fetchedAt)}；与战绩列表场数无关`">
        <ScoutRing :value="data.periods[0]!.winRate" short="12h" label="12小时内" :caption="periodCaption(data.periods[0]!)" :detail="periodDetail(data.periods[0]!)" :title="`${data.queueLabel} · 最近12小时，胜负有效 ${data.periods[0]!.outcomeGames}/${data.periods[0]!.games} 场${data.periods[0]!.complete ? '' : '；仅已读范围'}`" />
        <span class="scout-position-cell">
          <ScoutRing :value="data.role.share" :role="data.role.key" short="" label="30天位置" :caption="roleCaption" :detail="data.role.known ? `${data.role.known} 场记录` : '—'" :title="roleTitle" />
          <span v-if="data.role.known" class="role-distribution" :title="roleTitle"><i v-for="p in data.role.positions" :key="p.position" :class="p.position.toLowerCase()" :style="{ flexGrow: p.games }" :title="`${roleNames[p.position]} ${p.games} 场`" /></span>
        </span>
        <ScoutRing :value="data.periods[1]!.winRate" short="30d" label="30天内" :caption="periodCaption(data.periods[1]!)" :detail="periodDetail(data.periods[1]!)" :title="`${data.queueLabel} · 最近30天，胜负有效 ${data.periods[1]!.outcomeGames}/${data.periods[1]!.games} 场${data.periods[1]!.complete ? '' : '；仅已读范围'}`" />
      </span>
      <span class="scout-scope" :title="`${data.queueLabel} · ${range}，${data.overall.games} 场；KDA 有效 ${data.overall.kdaGames ?? 0} 场；统计截至 ${date(data.report?.fetchedAt)}；${data.report?.season?.name || '赛季范围尚未取得'}`"><small class="sample-size">{{ range }}</small><span>KDA <b>{{ decimal(data.overall.kda) }}</b></span><span v-if="data.report && !data.report.complete30" class="scout-coverage-mark">范围未完整</span></span>
      <span class="scout-tags recent-block"><span v-for="tag in data.tags" :key="tag.label" class="scout-tag" :class="tag.tone" :title="tag.title">{{ tag.label }}</span><small v-if="!data.tags.length" class="scout-empty-tags">{{ data.report?.state === 'loading' || !data.report ? captured ? '本局未保留排位统计' : '正在准备排位统计' : '本季英雄记录' }}</small></span>
    </template>
    <span v-else class="rich-state scout-state"><AppIcon name="history" :size="24" /><b>身份未提供</b><small>等待玩家身份</small></span>
  </span>
</template>
