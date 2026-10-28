import { shallowRef } from 'vue';
import { Events } from '@wailsio/runtime';
import * as Service from '../bindings/leaguepingo/appservice';
import type { State } from '../bindings/leaguepingo/internal/guides/models';
const state=shallowRef<State|null>(null);let started=false;
export function acceptGuide(value:State|undefined){if(value&&(!state.value||value.revision>=state.value.revision))state.value=value;}
export function useGuide(){
 if(!started){started=true;Events.On('leaguepingo:guide',e=>acceptGuide(e.data as State));void Service.GuideStatus().then(acceptGuide).catch(()=>{});}
 return state;
}
