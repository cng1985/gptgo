<script setup>
import { onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import Sidebar from '../components/Sidebar.vue'
import { boot, chat, notify, resetDraft, stop } from '../stores/chat'

const router = useRouter()

function onKeydown(event) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    stop()
    resetDraft()
    router.push('/')
  }
}

onMounted(async () => {
  window.addEventListener('keydown', onKeydown)
  try {
    await boot()
  } catch (error) {
    if (error.code === 401) {
      router.push('/login')
      return
    }
    notify(error.message || '加载失败')
  }
})

onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="shell">
    <div v-if="chat.sidebarOpen" class="backdrop" @click="chat.sidebarOpen = false" />
    <Sidebar :class="{ open: chat.sidebarOpen }" />
    <section class="main">
      <router-view />
    </section>
    <div v-if="chat.toast" class="toast">{{ chat.toast }}</div>
  </div>
</template>
