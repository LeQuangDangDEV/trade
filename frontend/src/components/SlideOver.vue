<template>
  <transition name="fade">
    <div v-if="open" class="backdrop" @click="onBackdrop"></div>
  </transition>

  <transition name="slide">
    <aside v-if="open" class="panel" role="dialog" aria-modal="true">
      <header class="panel__header">
        <slot name="title"><h3>{{ title }}</h3></slot>
        <button class="panel__close" @click="$emit('close')">×</button>
      </header>
      <div class="panel__body">
        <slot />
      </div>
    </aside>
  </transition>
</template>

<script setup lang="ts">
defineProps<{ open: boolean; title?: string; closeOnBackdrop?: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()
function onBackdrop() { emit('close') }
</script>

<style scoped>
.backdrop { position: fixed; inset: 0; background: rgba(0,0,0,.25); }
.panel {
  position: fixed; top: 0; right: 0; height: 100vh;
  width: min(420px, 92vw); background: #fff; box-shadow: -4px 0 20px rgba(0,0,0,.15);
  display:flex; flex-direction:column;
}
.panel__header { display:flex; align-items:center; justify-content:space-between; padding:12px 16px; border-bottom:1px solid #eee; }
.panel__close { font-size:24px; line-height:1; background:transparent; border:none; cursor:pointer; }
.panel__body { padding:16px; overflow:auto; }
.fade-enter-active, .fade-leave-active { transition: opacity .2s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
.slide-enter-active, .slide-leave-active { transition: transform .25s; }
.slide-enter-from, .slide-leave-to { transform: translateX(100%); }
</style>
