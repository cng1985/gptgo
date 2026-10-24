<script setup>
import { nextTick, onMounted, ref, watch } from 'vue'
import Icon from './Icon.vue'
import SparkleMark from './SparkleMark.vue'
import { chat, notify, send } from '../stores/chat'
import { renderMarkdown } from '../utils/markdown'

const scroller = ref(null)
const stick = ref(true)
const editingId = ref('')
const editingText = ref('')

function html(message) {
  return renderMarkdown(message.content || '')
}

async function scrollIfNeeded(force = false) {
  await nextTick()
  const el = scroller.value
  if (!el || (!stick.value && !force)) return
  el.scrollTop = el.scrollHeight
}

function onScroll() {
  const el = scroller.value
  if (!el) return
  stick.value = el.scrollHeight - el.scrollTop - el.clientHeight < 80
}

async function copyMessage(message) {
  try {
    await navigator.clipboard.writeText(message.content || '')
    notify('已复制')
  } catch {
    notify('复制失败')
  }
}

function onCopyClick(event) {
  const button = event.target.closest('.copy-code')
  if (!button) return
  const code = button.parentElement?.nextElementSibling?.innerText || ''
  navigator.clipboard.writeText(code).then(() => notify('已复制代码')).catch(() => notify('复制失败'))
}

function startEdit(message) {
  editingId.value = String(message.id)
  editingText.value = message.content
}

function commitEdit(message) {
  const text = editingText.value.trim()
  editingId.value = ''
  if (!text || text === message.content) return
  send({ content: text, editId: message.id })
}

watch(() => chat.messages.map((item) => item.content + item.thinking + item.status).join('|'), () => scrollIfNeeded())
onMounted(() => scrollIfNeeded(true))
</script>

<template>
  <div ref="scroller" class="messages" @scroll="onScroll" @click="onCopyClick">
    <article v-for="message in chat.messages" :key="message.id" class="message" :class="message.role">
      <div v-if="message.role === 'assistant'" class="assistant-mark"><SparkleMark :size="16" /></div>
      <div class="bubble-wrap">
        <div v-if="message.role === 'assistant' && message.thinking" class="thinking">
          <details :open="message.status === 'streaming'">
            <summary>{{ message.status === 'streaming' ? '思考中' : '已思考' }}</summary>
            <p>{{ message.thinking }}</p>
          </details>
        </div>
        <div v-if="message.attachments?.length" class="msg-files">
          <span v-for="file in message.attachments" :key="file.id">{{ file.name }}</span>
        </div>
        <textarea v-if="editingId === String(message.id)" v-model="editingText" class="edit-box" rows="3" />
        <div v-else-if="message.role === 'user'" class="user-bubble">{{ message.content }}</div>
        <div v-else class="markdown" v-html="html(message)" />
        <span v-if="message.role === 'assistant' && message.status === 'streaming'" class="caret" />
        <div v-if="message.role === 'assistant' && message.status !== 'streaming'" class="message-actions">
          <button type="button" aria-label="复制" @click="copyMessage(message)"><Icon name="copy" :size="15" /></button>
          <button type="button" aria-label="重新生成" @click="send({ regenerateId: message.id })"><Icon name="refresh" :size="15" /></button>
        </div>
        <div v-if="message.role === 'user' && message.status !== 'streaming'" class="message-actions user-actions">
          <button v-if="editingId !== String(message.id)" type="button" @click="startEdit(message)">编辑</button>
          <button v-else type="button" @click="commitEdit(message)">发送</button>
        </div>
      </div>
    </article>
  </div>
</template>
