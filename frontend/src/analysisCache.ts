import { ref } from "vue";
import type {MatchDetail,MatchTimeline} from '../bindings/leaguepingo/internal/core/models';
// Ephemeral UI cache only. Keys include the verified connection and player;
// backend enforces the same scope and expiry before returning new results.
const details=new Map<string,{at:number;value:MatchDetail}>();
const timelines=new Map<string,{at:number;value:MatchTimeline}>();
const detailRequests = new Map<string, Promise<MatchDetail>>();
const detailRevision = ref(0);
const ttl=5*60*1000;
function read<T>(map:Map<string,{at:number;value:T}>,key:string){const item=map.get(key);if(item&&Date.now()-item.at<ttl)return item.value;map.delete(key);return null;}
function write<T>(map:Map<string,{at:number;value:T}>,key:string,value:T,limit:number){map.set(key,{at:Date.now(),value});while(map.size>limit)map.delete(map.keys().next().value!);}
function fetchDetail(key: string, fetch: () => PromiseLike<MatchDetail>) {
  const cached = read(details, key);
  if (cached) return Promise.resolve(cached);
  const active = detailRequests.get(key);
  if (active) return active;

  const request = Promise.resolve().then(fetch).then(detail => {
    if (detail.players?.length) { write(details, key, detail, 32); detailRevision.value++; }
    return detail;
  }).finally(() => detailRequests.delete(key));
  detailRequests.set(key, request);
  return request;
}

export const analysisCache = {
  detail: (key: string) => read(details, key),
  timeline: (key: string) => read(timelines, key),
  fetchDetail,
  setDetail: (key: string, value: MatchDetail) => { write(details, key, value, 32); detailRevision.value++; },
  roster: (connection: number, game: number) => {
    void detailRevision.value;
    for (const [key] of details) {
      const detail = read(details, key);
      if (detail?.connectionId === connection && detail.gameId === game) return detail.players;
    }
    return [];
  },
  setTimeline: (key: string, value: MatchTimeline) => write(timelines, key, value, 8),
};

type TimelineCall = PromiseLike<MatchTimeline> & { cancel?: () => unknown };
const activeTimelines = new Map<string, { call: TimelineCall; promise: Promise<MatchTimeline>; users: number }>();
export function acquireTimeline(key: string, fetch: () => TimelineCall) {
  let task = activeTimelines.get(key);
  if (!task) {
    const call = fetch();
    const next = { call, users: 0, promise: Promise.resolve(call).finally(() => {
      if (activeTimelines.get(key) === next) activeTimelines.delete(key);
    }) };
    task = next;
    activeTimelines.set(key, task);
  }
  task.users++;
  let released = false;
  return { promise: task.promise, release: () => {
    if (released) return;
    released = true;
    task.users--;
    if (!task.users && activeTimelines.get(key) === task) {
      activeTimelines.delete(key);
      task.call.cancel?.();
    }
  } };
}
