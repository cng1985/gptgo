<script setup>
import { ref } from 'vue'
import { chat } from '../stores/chat'

const open = ref(true)

function askMascot() {
  chat.draft = '团子，帮我把当前问题整理成可以直接执行的三步。'
  open.value = true
}
</script>

<template>
  <div class="mascot">
    <button v-if="open" class="mascot-bubble" type="button" @click="askMascot">
      <strong>团子</strong>
      <span>有什么可以帮你?</span>
    </button>
    <button class="mascot-face" type="button" aria-label="团子" @click="open = !open">
      <svg viewBox="0 0 120 120" aria-hidden="true">
        <defs>
          <linearGradient id="bun" x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stop-color="#f7f3ff" />
            <stop offset="1" stop-color="#d9d2ff" />
          </linearGradient>
        </defs>
        <ellipse cx="86" cy="78" rx="18" ry="14" fill="#efe8ff" />
        <path d="M34 28c2-12 18-16 26-6 8-12 26-8 28 6 10 2 16 14 12 24-2 18-16 28-34 28S24 72 24 54c-2-12 4-24 10-26z" fill="url(#bun)" />
        <path d="M40 30c2-10 12-14 16-4" fill="none" stroke="#c8bdf8" stroke-width="3" />
        <path d="M68 24c6-8 16-6 18 4" fill="none" stroke="#c8bdf8" stroke-width="3" />
        <ellipse cx="48" cy="54" rx="16" ry="14" fill="#fff" />
        <ellipse cx="74" cy="54" rx="16" ry="14" fill="#fff" />
        <path d="M42 54c2 4 8 4 12 0" fill="none" stroke="#6d64c8" stroke-width="2.4" stroke-linecap="round" />
        <path d="M68 54c2 4 8 4 12 0" fill="none" stroke="#6d64c8" stroke-width="2.4" stroke-linecap="round" />
        <path d="M58 64c3 4 8 4 11 0" fill="none" stroke="#8a78d8" stroke-width="2" stroke-linecap="round" />
        <circle cx="40" cy="62" r="3" fill="#f3b7d8" opacity=".8" />
        <circle cx="84" cy="62" r="3" fill="#f3b7d8" opacity=".8" />
        <path d="M52 34c6-8 14-8 18 0" fill="none" stroke="#9b8ef0" stroke-width="2" />
        <circle cx="96" cy="96" r="7" fill="#3dce73" stroke="#fff" stroke-width="3" />
      </svg>
    </button>
  </div>
</template>
