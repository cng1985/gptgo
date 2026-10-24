<script setup>
import Icon from './Icon.vue'
import { chat, toggleTheme } from '../stores/chat'

defineProps({
  title: { type: String, required: true },
})
</script>

<template>
  <header class="topbar">
    <button class="icon-button mobile-only" type="button" aria-label="打开侧栏" @click="chat.sidebarOpen = true">
      <Icon name="menu" />
    </button>
    <strong class="page-title">{{ title }}</strong>
    <div class="top-actions">
      <button class="icon-button" type="button" aria-label="切换外观" @click="toggleTheme">
        <Icon :name="chat.theme === 'dark' ? 'sun' : 'moon'" />
      </button>
    </div>
  </header>
</template>
