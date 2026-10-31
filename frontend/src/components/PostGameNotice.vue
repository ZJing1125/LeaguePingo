<script setup lang="ts">
// Same rule as the accept notice: no static "已开启" banner, only a step that is
// running, counting down, or waiting for the player.
import { useAutomation } from '../composables/useAutomation';
defineProps<{ compact?: boolean }>();
const { postVisible, postText, post, cancelPost, busy, error } = useAutomation();
const cancelable = ['honor-countdown', 'waiting-honor', 'return-countdown', 'queue-countdown', 'waiting-result', 'waiting-return', 'waiting-ready', 'waiting-postgame', 'continuing-postgame'];
</script>
<template>
  <div v-if="postVisible" class="post-game-notice" :class="{ compact }" :data-status="post?.status">
    <span role="status">{{ error || postText }}</span>
    <button v-if="cancelable.includes(post?.status ?? '')" class="secondary" :disabled="busy" @click="cancelPost">取消本次</button>
  </div>
</template>
<style scoped>
.post-game-notice{display:flex;align-items:center;justify-content:space-between;gap:8px;min-height:32px;margin:0 0 12px;padding:6px 10px;border:1px solid var(--border);border-radius:6px;color:var(--text);font-size:12px;font-variant-numeric:tabular-nums;}
.post-game-notice[data-status="failed"],.post-game-notice[data-status="timeout"],.post-game-notice[data-status="honor-unavailable"],.post-game-notice[data-status="honor-unconfirmed"],.post-game-notice[data-status="honor-mismatch"],.post-game-notice[data-status="continue-unavailable"],.post-game-notice[data-status="continue-unconfirmed"]{color:var(--danger, #e06c6c);}
.post-game-notice button{flex:none;min-height:28px;padding:3px 8px;font-size:12px;}
.post-game-notice.workspace-message{margin:8px 20px 0;flex:none;}
.compact{margin:0 8px 8px;flex-wrap:wrap;}
</style>
