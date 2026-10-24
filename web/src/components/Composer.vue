<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import Icon from './Icon.vue'
import { findAgent } from '../stores/agent'
import { addFiles, chat, optimizeDraft, removeAttachment, send, stop } from '../stores/chat'

const box = ref(null)
const fileRef = ref(null)
let recognition = null
const placeholder = computed(() => {
  const agent = findAgent(chat.agentId)
  return agent ? `给${agent.name}发送消息` : '给 AI 发送消息'
})

watch(
  () => chat.draft,
  async () => {
    await nextTick()
    const el = box.value
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, 180)}px`
  },
)

function onKeydown(event) {
  if (event.key === 'Enter' && !event.shiftKey && !event.isComposing && event.keyCode !== 229) {
    event.preventDefault()
    submit()
  }
}

function submit() {
  if (chat.streaming) {
    stop()
    return
  }
  send()
}

function pickFiles() {
  fileRef.value?.click()
}

async function onFiles(event) {
  await addFiles(event.target.files)
  event.target.value = ''
}

function onDrop(event) {
  addFiles(event.dataTransfer?.files)
}

function toggleMic() {
  const Speech = window.SpeechRecognition || window.webkitSpeechRecognition
  if (!Speech) {
    chat.toast = '当前浏览器不能使用语音输入'
    return
  }
  if (recognition) {
    recognition.stop()
    recognition = null
    return
  }
  recognition = new Speech()
  recognition.lang = 'zh-CN'
  recognition.interimResults = false
  recognition.onresult = (event) => {
    const text = event.results?.[0]?.[0]?.transcript || ''
    chat.draft = `${chat.draft} ${text}`.trim()
  }
  recognition.onerror = () => {
    chat.toast = '没有听清，请再试一次'
  }
  recognition.onend = () => {
    recognition = null
  }
  recognition.start()
}
</script>

<template>
  <div class="composer-wrap" @dragover.prevent @drop.prevent="onDrop">
    <div class="composer" :class="{ drag: false }">
      <div v-if="chat.attachments.length" class="file-chips">
        <span v-for="(file, index) in chat.attachments" :key="file.name + index">
          {{ file.name }}
          <button type="button" aria-label="移除文件" @click="removeAttachment(index)">×</button>
        </span>
      </div>
      <textarea
        ref="box"
        v-model="chat.draft"
        rows="1"
        :placeholder="placeholder"
        @keydown="onKeydown"
      />
      <div class="composer-bar">
        <div class="composer-left">
          <button class="circle-tool" type="button" aria-label="添加文件" @click="pickFiles">
            <Icon name="paperclip" :size="16" />
          </button>
          <button class="opt-chip" type="button" :disabled="chat.optimizing" @click="optimizeDraft">
            <Icon name="spark" :size="15" />
            {{ chat.optimizing ? '优化中' : '优化提示词' }}
          </button>
          <span class="file-hint">支持文本与代码文件</span>
          <input ref="fileRef" class="hidden-file" type="file" multiple accept=".txt,.md,.json,.yaml,.yml,.go,.java,.sql,.js,.ts,.tsx,.jsx,.vue,.py,.sh,.xml,.csv,.html,.css,.c,.cpp,.h,.rs,.toml,.ini,.log,.php,.rb,.mod,.sum,.ps1,.cmd,text/plain" @change="onFiles" />
        </div>
        <div class="composer-right">
          <button class="circle-tool quiet" type="button" aria-label="语音输入" @click="toggleMic">
            <Icon name="mic" :size="16" />
          </button>
          <button class="send" :class="{ ready: chat.draft.trim() || chat.attachments.length || chat.streaming }" type="button" :aria-label="chat.streaming ? '停止生成' : '发送'" @click="submit">
            <Icon :name="chat.streaming ? 'stop' : 'arrow-up'" :size="16" />
          </button>
        </div>
      </div>
    </div>
    <p class="disclaimer">AI 可能会出错，请核查重要信息。Enter 发送，Shift + Enter 换行。</p>
  </div>
</template>
