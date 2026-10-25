<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Icon from './Icon.vue'
import {
  chat,
  clearConversations,
  models,
  notify,
  setModel,
  setThinking,
  thinkingModes,
  toggleTheme,
  activeTitle,
} from '../stores/chat'

const modelOpen = ref(false)
const thinkOpen = ref(false)
const settingsOpen = ref(false)
const confirmClear = ref(false)

const modelLabel = computed(() => models.find((item) => item.id === chat.model)?.label || 'MiniMax-M3.1-Flash')
const thinkingLabel = computed(() => thinkingModes.find((item) => item.id === chat.thinking)?.label || '不思考 · mini')

function chooseModel(id) {
  setModel(id)
  modelOpen.value = false
}

function chooseThinking(id) {
  setThinking(id)
  thinkOpen.value = false
}

function downloadChat() {
  if (!chat.messages.length) {
    notify('当前没有可导出的对话')
    return
  }
  const lines = [`# ${activeTitle()}`, '']
  for (const message of chat.messages) {
    lines.push(message.role === 'user' ? '## 你' : '## ChatBI AI', '', message.content || '')
    if (message.role === 'assistant' && message.sources?.length) {
      lines.push('', '来源：')
      for (const source of message.sources) {
        lines.push(`- ${source.title || source.url} ${source.url || ''}`.trim())
      }
    }
    lines.push('')
  }
  const blob = new Blob([lines.join('\n')], { type: 'text/markdown;charset=utf-8' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = `${activeTitle() || '对话'}.md`
  link.click()
  URL.revokeObjectURL(link.href)
  notify('已开始下载')
}

async function clearAll() {
  try {
    await clearConversations()
    confirmClear.value = false
    settingsOpen.value = false
  } catch (error) {
    notify(error.message || '清空失败')
  }
}

function closeMenus() {
  modelOpen.value = false
  thinkOpen.value = false
  settingsOpen.value = false
}

function eventTarget(event) {
  if (event.target instanceof Element) return event.target
  return event.target?.parentElement || null
}

function onPointerDown(event) {
  if (eventTarget(event)?.closest('.picker')) return
  closeMenus()
}

function onKeydown(event) {
  if (event.key === 'Escape') closeMenus()
}

onMounted(() => {
  document.addEventListener('pointerdown', onPointerDown, true)
  document.addEventListener('click', onPointerDown, true)
  document.addEventListener('keydown', onKeydown)
})
onUnmounted(() => {
  document.removeEventListener('pointerdown', onPointerDown, true)
  document.removeEventListener('click', onPointerDown, true)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <header class="topbar" @pointerdown.self="closeMenus">
    <button class="icon-button mobile-only" type="button" aria-label="打开侧栏" @click="chat.sidebarOpen = true">
      <Icon name="menu" />
    </button>
    <div class="picker" :class="{ open: modelOpen }">
      <button class="model-trigger" type="button" @click="modelOpen = !modelOpen; thinkOpen = false; settingsOpen = false">
        <span>{{ modelLabel }}</span>
        <Icon name="chevron" :size="16" />
      </button>
      <div v-if="modelOpen" class="menu model-menu">
        <button v-for="item in models" :key="item.id" type="button" :class="{ selected: item.id === chat.model }" @click="chooseModel(item.id)">
          <span>{{ item.label }}</span>
          <Icon v-if="item.id === chat.model" name="check" :size="16" />
        </button>
      </div>
    </div>
    <div class="picker">
      <button class="think-trigger" type="button" @click="thinkOpen = !thinkOpen; modelOpen = false; settingsOpen = false">
        <Icon name="spark" :size="15" />
        <span>{{ thinkingLabel }}</span>
        <Icon name="chevron" :size="14" />
      </button>
      <div v-if="thinkOpen" class="menu think-menu">
        <button v-for="item in thinkingModes" :key="item.id" type="button" class="think-option" :class="{ selected: item.id === chat.thinking }" @click="chooseThinking(item.id)">
          <span class="think-option-copy">
            <strong>{{ item.label }}</strong>
            <small>{{ item.hint }}</small>
          </span>
          <Icon v-if="item.id === chat.thinking" name="check" :size="16" />
        </button>
      </div>
    </div>
    <div class="top-actions">
      <button class="icon-button" type="button" aria-label="切换外观" @click="toggleTheme">
        <Icon :name="chat.theme === 'dark' ? 'sun' : 'moon'" />
      </button>
      <button class="icon-button" type="button" aria-label="导出对话" @click="downloadChat">
        <Icon name="download" />
      </button>
      <div class="picker">
        <button class="icon-button" type="button" aria-label="设置" @click="settingsOpen = !settingsOpen; modelOpen = false; thinkOpen = false">
          <Icon name="gear" />
        </button>
        <div v-if="settingsOpen" class="menu settings-menu">
          <p><strong>模型来源</strong></p>
          <p class="muted">{{ chat.account.provider_ready ? '已连接外部模型接口' : '当前使用内置助手。配置 APP_AI_API_KEY 后可接入 OpenAI 兼容接口。' }}</p>
          <p class="muted">打开输入框里的「联网」后，问题会先检索公开网页，再阅读靠前的结果。默认是 DuckDuckGo，打不开时改用 Lite，并补充中文维基百科。</p>
          <p><strong>快捷键</strong></p>
          <p class="muted">Ctrl K 新建对话。Enter 发送，Shift + Enter 换行。</p>
          <button v-if="!confirmClear" type="button" class="danger" @click="confirmClear = true">清空所有对话</button>
          <div v-else class="confirm-row">
            <button type="button" @click="confirmClear = false">取消</button>
            <button type="button" class="danger" @click="clearAll">确认清空</button>
          </div>
        </div>
      </div>
    </div>
  </header>
</template>
