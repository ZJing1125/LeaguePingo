import type {MatchDetail,MatchTimeline} from '../bindings/leaguepingo/internal/core/models';
// Ephemeral UI cache only. Keys include the verified connection and player;
// backend enforces the same scope and expiry before returning new results.
const details=new Map<string,{at:number;value:MatchDetail}>();
const timelines=new Map<string,{at:number;value:MatchTimeline}>();
const ttl=5*60*1000;
function read<T>(map:Map<string,{at:number;value:T}>,key:string){const item=map.get(key);if(item&&Date.now()-item.at<ttl)return item.value;map.delete(key);return null;}
function write<T>(map:Map<string,{at:number;value:T}>,key:string,value:T,limit:number){map.set(key,{at:Date.now(),value});while(map.size>limit)map.delete(map.keys().next().value!);}
export const analysisCache={detail:(key:string)=>read(details,key),timeline:(key:string)=>read(timelines,key),setDetail:(key:string,value:MatchDetail)=>write(details,key,value,32),setTimeline:(key:string,value:MatchTimeline)=>write(timelines,key,value,8)};
