<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import TopBar from '../components/TopBar.vue'
import { chat, resetDraft, stop } from '../stores/chat'
import { groupLabel } from '../utils/time'

const router = useRouter()
const recent = computed(() => chat.conversations.slice(0, 6))

function openItem(id) {
  router.push(`/c/${id}`)
}

function newChat() {
  stop()
  resetDraft()
  router.push('/')
}
</script>

<template>
  <div class="workspace">
    <TopBar />
    <div class="workspace-body">
      <h1>工作台</h1>
      <p>从这里回到对话，或接着上次的问题往下做。</p>
      <div class="workspace-actions">
        <button type="button" class="primary" @click="newChat">开始新对话</button>
      </div>
      <div class="workspace-grid">
        <button v-for="item in recent" :key="item.id" type="button" @click="openItem(item.id)">
          <strong>{{ item.title }}</strong>
          <span>{{ groupLabel(item.updated_at) }}</span>
        </button>
        <p v-if="!recent.length" class="muted">还没有对话。</p>
      </div>
    </div>
  </div>
</template>
