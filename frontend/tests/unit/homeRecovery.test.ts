import { afterEach, beforeEach, expect, test, vi } from 'vitest';
import { createRenderer, defineComponent, nextTick, ref } from 'vue';
import * as Service from '../../bindings/leaguepingo/appservice';
import { useHomePlayer } from '../../src/composables/useHomePlayer';
import type { Participant, Snapshot } from '../../bindings/leaguepingo/internal/core/models';

vi.mock('../../bindings/leaguepingo/appservice', () => ({CurrentPlayer:vi.fn(),EnsureProfile:vi.fn()}));
const renderer = createRenderer<object, object>({
 createElement:()=>({}),createText:()=>({}),createComment:()=>({}),insert(){},remove(){},
 setText(){},setElementText(){},patchProp(){},parentNode:()=>null,nextSibling:()=>null,
});
let close: (()=>void) | undefined;
function setup() {
 const snapshot=ref({connected:true,connectionId:1,selfKey:'self',stage:'idle',participants:[]} as unknown as Snapshot);
 const view=ref('home'),busy=ref(false);
 let home!: ReturnType<typeof useHomePlayer>;
 const app=renderer.createApp(defineComponent({setup(){home=useHomePlayer(snapshot,view,busy);return()=>null;}}));
 app.mount({});close=()=>app.unmount();
 return {snapshot,view,busy,home};
}
const player = {key:'self',puuid:'self',connectionId:1,displayName:'Ready#TEST',history:[]} as unknown as Participant;
beforeEach(()=>{vi.useFakeTimers();vi.resetAllMocks();vi.mocked(Service.EnsureProfile).mockResolvedValue(player);});
afterEach(()=>{close?.();vi.useRealTimers();});

test('initial failure recovers automatically and stops after success',async()=>{
 vi.mocked(Service.CurrentPlayer).mockRejectedValueOnce(new Error('starting')).mockResolvedValue(player);
 const {home}=setup();await nextTick();await vi.advanceTimersByTimeAsync(2000);
 expect(home.player.value?.displayName).toBe('Ready#TEST');
 await vi.advanceTimersByTimeAsync(60000);
 expect(Service.CurrentPlayer).toHaveBeenCalledTimes(2);
});
test('persistent startup failures stop after two retries',async()=>{
 vi.mocked(Service.CurrentPlayer).mockRejectedValue(new Error('starting'));
 setup();await vi.advanceTimersByTimeAsync(60000);
 expect(Service.CurrentPlayer).toHaveBeenCalledTimes(3);
});
test('busy or another view suspends retry without adding requests',async()=>{
 vi.mocked(Service.CurrentPlayer).mockRejectedValueOnce(new Error('starting')).mockResolvedValue(player);
 const {view,busy}=setup();await nextTick();view.value='players';await nextTick();
 await vi.advanceTimersByTimeAsync(3000);expect(Service.CurrentPlayer).toHaveBeenCalledTimes(1);
 view.value='home';busy.value=true;await nextTick();expect(Service.CurrentPlayer).toHaveBeenCalledTimes(1);
 busy.value=false;await nextTick();await vi.advanceTimersByTimeAsync(0);
 expect(Service.CurrentPlayer).toHaveBeenCalledTimes(2);
});
test('new connection starts its own budget and rejects late old results',async()=>{
 let release!:(p:Participant)=>void;
 const cancel=vi.fn();
 vi.mocked(Service.CurrentPlayer).mockImplementationOnce(()=>Object.assign(new Promise<Participant>(resolve=>{release=resolve;}),{cancel}) as unknown as ReturnType<typeof Service.CurrentPlayer>);
 vi.mocked(Service.CurrentPlayer).mockResolvedValue({...player,key:'next',puuid:'next',connectionId:2} as Participant);
 const {snapshot,home}=setup();snapshot.value={...snapshot.value,connectionId:2,selfKey:'next'};
 await nextTick();await vi.advanceTimersByTimeAsync(0);release(player);await nextTick();
 expect(home.player.value?.key).toBe('next');expect(Service.CurrentPlayer).toHaveBeenCalledTimes(2);
 expect(cancel).toHaveBeenCalledOnce();
});
test('profile recovery preserves prepared history without reloading it',async()=>{
 vi.mocked(Service.CurrentPlayer).mockResolvedValue(player);
 vi.mocked(Service.EnsureProfile).mockRejectedValueOnce(new Error('profile starting')).mockResolvedValue(player);
 const {home}=setup();await vi.advanceTimersByTimeAsync(60000);
 expect(home.player.value?.key).toBe('self');
 expect(Service.CurrentPlayer).toHaveBeenCalledTimes(1);
 expect(Service.EnsureProfile).toHaveBeenCalledTimes(2);
});
test('expired recovery waits for explicit retry',async()=>{
 vi.mocked(Service.CurrentPlayer).mockRejectedValue(new Error('starting'));
 const {view,home}=setup();await nextTick();view.value='players';await nextTick();
 await vi.advanceTimersByTimeAsync(61000);view.value='home';await nextTick();
 expect(Service.CurrentPlayer).toHaveBeenCalledTimes(1);
 await home.retry();expect(Service.CurrentPlayer).toHaveBeenCalledTimes(2);
});
