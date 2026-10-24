<script setup>
import { onMounted, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import Composer from '../components/Composer.vue'
import EmptyHero from '../components/EmptyHero.vue'
import MessageList from '../components/MessageList.vue'
import TopBar from '../components/TopBar.vue'
import { chat, notify, openConversation, stop } from '../stores/chat'

const route = useRoute()
const router = useRouter()

async function syncRoute(id) {
  if (chat.streaming && id && String(chat.activeId) === String(id)) return
  if (!id) {
    if (!chat.streaming) {
      chat.activeId = ''
      chat.messages = []
      chat.agentId = route.query.agent ? String(route.query.agent) : ''
    }
    return
  }
  if (String(chat.activeId) === String(id) && chat.messages.length) return
  try {
    await openConversation(id)
  } catch (error) {
    notify(error.message || '对话不存在')
    router.replace('/')
  }
}

watch(() => [route.params.id, route.query.agent], () => {
  syncRoute(route.params.id)
})

watch(() => chat.activeId, (id) => {
  if (!chat.streaming || !id) return
  if (String(route.params.id) === String(id)) return
  router.replace(`/c/${id}`)
})

onMounted(() => syncRoute(route.params.id))
onBeforeRouteLeave((to) => {
  if (to.name !== 'conversation' && to.name !== 'new-chat') stop()
})
</script>

<template>
  <div class="chat-screen">
    <TopBar />
    <div class="chat-stage">
      <EmptyHero v-if="!chat.messages.length" />
      <MessageList v-else />
      <Composer />
    </div>
  </div>
</template>
