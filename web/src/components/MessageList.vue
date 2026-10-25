<script setup>
import { nextTick, onMounted, ref, watch } from 'vue'
import ChartPanel from './ChartPanel.vue'
import Icon from './Icon.vue'
import SparkleMark from './SparkleMark.vue'
import ThinkingBlock from './ThinkingBlock.vue'
import { chat, notify, readSource, send } from '../stores/chat'
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

function safeLink(url) {
  try {
    const parsed = new URL(url)
    if (parsed.protocol === 'http:' || parsed.protocol === 'https:') return parsed.href
  } catch {
    return ''
  }
  return ''
}

function hostOf(url) {
  try {
    return new URL(url).hostname.replace(/^www\./, '')
  } catch {
    return ''
  }
}

function sourceText(item) {
  return String(item?.excerpt || item?.snippet || '').trim()
}

function sourceMeta(item) {
  const host = hostOf(item.url)
  const engine = String(item?.engine || '').trim()
  if (engine && host) return `${engine} · ${host}`
  return engine || host
}

function searchHeadline(message) {
  const query = String(message.searchQuery || '').trim()
  if (message.searchStatus === 'running') {
    return query ? `正在检索「${query}」，并打开结果页读取正文` : '正在检索公开网页，并打开结果页'
  }
  if (message.searchStatus === 'empty' || message.searchStatus === 'error') {
    return query ? `没有检索到「${query}」的公开结果。` : '这次没有检索到可用的公开结果。'
  }
  return query ? `联网检索 · ${query}` : '联网检索'
}

function toggleDetail(message, item) {
  if (item.detailOpen) {
    item.detailOpen = false
    return
  }
  if (item.excerpt) {
    item.detailOpen = true
    return
  }
  readSource(message, item)
}

function detailLabel(item) {
  if (item.reading) return '正在读取…'
  if (item.excerpt && !item.detailOpen) return '查看明细'
  if (item.excerpt) return '重新读取'
  return '读取明细'
}

function openDetail(message, item) {
  if (item.excerpt && !item.detailOpen) {
    item.detailOpen = true
    return
  }
  readSource(message, item)
}

watch(() => chat.messages.map((item) => `${item.content}|${item.thinking}|${item.status}|${item.searchStatus || ''}|${item.searchQuery || ''}|${item.chart?.type || ''}|${item.chart?.rows?.length || 0}|${(item.sources || []).map((source) => `${source.url || ''}:${source.snippet || ''}:${source.excerpt || ''}:${source.detailOpen ? 1 : 0}:${source.reading ? 1 : 0}`).join(',')}|${(item.tools || []).map((tool) => `${tool.id}:${tool.status}:${tool.result || ''}`).join(',')}`).join('\n'), () => scrollIfNeeded())
onMounted(() => scrollIfNeeded(true))
</script>

<template>
  <div ref="scroller" class="messages" @scroll="onScroll" @click="onCopyClick">
    <article v-for="message in chat.messages" :key="message.id" class="message" :class="message.role">
      <div v-if="message.role === 'assistant'" class="assistant-mark"><SparkleMark :size="16" /></div>
      <div class="bubble-wrap">
        <div v-if="message.role === 'assistant' && message.tools?.length" class="tool-trace">
          <details v-for="tool in message.tools" :key="tool.id || tool.name" :open="tool.status === 'running' || message.status === 'streaming'">
            <summary>{{ tool.status === 'running' ? '正在调用' : (tool.ok === false ? '调用失败' : '已调用') }} {{ tool.name }}</summary>
            <p v-if="tool.arguments" class="tool-args">{{ tool.arguments }}</p>
            <p v-if="tool.result">{{ tool.result }}</p>
          </details>
        </div>
        <div v-if="message.role === 'assistant' && (message.searchStatus || message.sources?.length)" class="search-panel">
          <div class="search-head">
            <Icon name="globe" :size="14" />
            <span>{{ searchHeadline(message) }}</span>
          </div>
          <template v-if="message.searchStatus !== 'running' && message.searchStatus !== 'empty' && message.searchStatus !== 'error'">
            <p v-if="message.sources?.length" class="search-hint">摘要不够时会自动打开结果链接；点「读取明细」可以再读一次更长正文。</p>
            <div v-for="(item, index) in message.sources" :key="item.url || index" class="search-hit">
              <button type="button" class="search-hit-main" :aria-expanded="item.detailOpen ? 'true' : 'false'" @click="toggleDetail(message, item)">
                <span class="search-hit-title">{{ item.title || item.url || '来源' }}</span>
                <span v-if="sourceMeta(item)" class="search-hit-meta">{{ sourceMeta(item) }}</span>
                <span v-if="sourceText(item)" class="search-hit-snippet">{{ sourceText(item) }}</span>
              </button>
              <div class="search-hit-actions">
                <button type="button" @click="openDetail(message, item)">{{ detailLabel(item) }}</button>
                <a v-if="safeLink(item.url)" :href="safeLink(item.url)" target="_blank" rel="noopener noreferrer">打开原页</a>
              </div>
              <div v-if="item.detailOpen" class="search-hit-detail">
                <p v-if="item.reading">正在打开这篇网页…</p>
                <p v-else-if="item.readError">{{ item.readError }}</p>
                <p v-else-if="item.excerpt">{{ item.excerpt }}</p>
                <p v-else>这篇网页还没有读到正文。</p>
              </div>
            </div>
          </template>
        </div>
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
        <template v-else>
          <ChartPanel v-if="message.chart" :chart="message.chart" />
          <div v-if="message.content" class="markdown" v-html="html(message)" />
        </template>
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
