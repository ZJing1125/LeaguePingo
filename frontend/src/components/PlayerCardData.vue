<script setup lang="ts">
import { computed } from "vue";
import type { Participant } from "../../bindings/leaguepingo/internal/core/models";
import { percent, decimal, preparationLabel, date } from "../presentation";
import { rankPresentation } from "../rankPresentation";
import { championName } from "../gameAssets";
import { scoutStats } from "../scoutStats";
import GameIcon from "./GameIcon.vue";
import AppIcon from "./AppIcon.vue";
import RankEmblem from "./RankEmblem.vue";
import ScoutRing from "./ScoutRing.vue";
const props = defineProps<{ player: Participant; frozen?: boolean; party?: boolean }>();
const data = computed(() => scoutStats(props.player.identityState === "known" ? props.player : { ...props.player, history: [] }));
const rank = computed(() => rankPresentation(props.player));
const season = computed(() => {
  if (props.player.identityState !== "known") return null;
  const r = rank.value.rank;
  return r?.wins != null && r.losses != null
    ? { wins: r.wins, losses: r.losses, rate: r.wins + r.losses > 0 ? r.wins * 100 / (r.wins + r.losses) : null }
    : null;
});
const hasHistory = computed(() => props.player.identityState === "known" && data.value.matches.length > 0);
const state = computed(() => props.player.identityState === "unavailable" ? "身份未提供"
  : props.player.historyState === "loading" ? "正在获取"
  : props.player.historyState === "failed" ? "可重试"
  : props.player.historyState === "empty" ? "暂无战绩" : preparationLabel(props.player, props.frozen));
const hero = computed(() => props.player.championId
  ? championName(props.player.championId).split("·").at(-1)
  : props.party ? "尚未选择英雄" : props.player.championState === "unselected" ? "尚未选择英雄" : "英雄待确认");
function periodDetail(p: ReturnType<typeof scoutStats>["periods"][number]) {
  if (!p.timed) return "时间暂缺";
  if (!p.games) return "暂无样本";
  return p.outcomeGames ? `${p.wins}胜 / ${p.losses}负` : "胜负暂缺";
}
</script>
<template>
  <span class="card-details scout-data">
    <span class="scout-headline">
      <span class="scout-champion">
        <span class="card-pick"><GameIcon v-if="player.championId" :id="player.championId" :size="32" /><AppIcon v-else name="match" :size="28" />
          <span><strong :title="hero">{{ hero }}</strong><small>{{ player.championState === 'intent' ? '预选英雄' : player.championId ? `本英雄 ${data.champion.games} 场` : party ? '房间成员' : '等待选择' }}</small></span>
        </span>
        <span v-if="data.champion.average" class="scout-kda" :title="`本英雄 ${data.champion.kdaGames} 场有效样本的平均 K / D / A`"><b>{{ data.champion.average[0]!.toFixed(1) }}</b> / <b>{{ data.champion.average[1]!.toFixed(1) }}</b> / <b>{{ data.champion.average[2]!.toFixed(1) }}</b></span>
        <span v-else class="scout-no-kda">{{ player.championId ? '本英雄样本不足' : '本英雄表现待定' }}</span>
        <small class="scout-hero-rate" :class="{ 'no-sample': !data.champion.outcomeGames }">{{ data.champion.outcomeGames ? percent(data.champion.winRate) : '暂无胜负样本' }}<span v-if="data.champion.outcomeGames"> · {{ data.champion.wins }}胜 / {{ data.champion.losses }}负</span></small>
      </span>
      <span class="scout-rank" :title="rank.title">
        <RankEmblem :tier="player.identityState === 'known' ? rank.rank?.tier : undefined" />
        <span class="rank-line"><b>{{ rank.label }}</b><small v-if="rank.retained">旧</small></span>
        <small class="scout-season">{{ season ? `${percent(season.rate)} · ${season.wins}胜 / ${season.losses}负` : '赛季胜负暂缺' }}</small>
      </span>
    </span>
    <template v-if="hasHistory">
      <span class="overview-metrics scout-rings" :title="`基于已取得的最近 ${data.overall.games} 场；时间段仅包含该样本，不代表完整时段记录。获取于 ${date(player.historyMeta?.fetchedAt)}`">
        <ScoutRing :value="data.periods[0]!.winRate" short="12h" label="12小时内" :caption="`样本 ${data.periods[0]!.games} 场`" :detail="periodDetail(data.periods[0]!)" :title="`最近 ${data.overall.games} 场中的 12 小时子样本，胜负有效 ${data.periods[0]!.outcomeGames}/${data.periods[0]!.games} 场；截止 ${date(player.historyMeta?.fetchedAt)}`" />
        <ScoutRing :value="data.role.share" :role="data.role.key" short="" label="近期位置" :caption="data.role.label" :detail="data.role.key ? `${data.role.count}/${data.role.known} 场` : data.role.known ? `${data.role.known} 场已记录` : '无位置样本'" title="仅统计历史明确提供的召唤师峡谷位置；至少 3 场、占比不少于一半且无并列；不代表本局分路" />
        <ScoutRing :value="data.periods[1]!.winRate" short="30d" label="30天内" :caption="`样本 ${data.periods[1]!.games} 场`" :detail="periodDetail(data.periods[1]!)" :title="`最近 ${data.overall.games} 场中的 30 天子样本，胜负有效 ${data.periods[1]!.outcomeGames}/${data.periods[1]!.games} 场；截止 ${date(player.historyMeta?.fetchedAt)}`" />
      </span>
      <span class="scout-scope" :title="`全部模式，胜负有效 ${data.overall.outcomeGames}/${data.overall.games} 场，KDA 有效 ${data.overall.kdaGames}/${data.overall.games} 场；时段统计仅限此样本`"><small class="sample-size">近 {{ data.overall.games }} 场内</small><b>{{ percent(data.overall.winRate) }}</b><span>{{ decimal(data.overall.kda) }} KDA</span></span>
      <span class="scout-tags recent-block"><span v-for="tag in data.tags" :key="tag.label" class="scout-tag" :class="tag.tone" :title="tag.title"><GameIcon v-if="tag.heroId" :id="tag.heroId" :size="12" />{{ tag.label }}</span></span>
    </template>
    <span v-else class="rich-state scout-state" :class="player.historyState"><AppIcon :name="player.historyState === 'loading' ? 'refresh' : 'history'" :size="24" /><b>{{ state }}</b><small>{{ player.identityState === 'unavailable' ? '等待玩家身份' : player.error || preparationLabel(player, frozen) }}</small><span v-if="player.privacyState === 'private'" class="scout-tag neutral">生涯不公开</span></span>
  </span>
</template>
