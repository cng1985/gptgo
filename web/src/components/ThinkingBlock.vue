<script setup>
import { computed, onUnmounted, ref, watch } from 'vue'
import Icon from './Icon.vue'

const props = defineProps({
  mode: { type: String, default: '' },
  text: { type: String, default: '' },
  streaming: { type: Boolean, default: false },
  variant: { type: String, default: 'chat' },
})

const open = ref(false)
const seconds = ref(0)
let timer = 0
let touched = false

const resolved = computed(() => {
  if (props.mode === 'off' || props.mode === 'light' || props.mode === 'deep') return props.mode
  return props.text ? 'light' : 'off'
})

const visible = computed(() => {
  if (resolved.value === 'off') return props.variant === 'ask' && Boolean(props.text)
  return props.streaming || Boolean(props.text)
})

const steps = computed(() => (props.text || '').split(/\n+/).map((line) => line.trim()).filter(Boolean))

const preview = computed(() => {
  const line = steps.value[0] || ''
  return line.length > 42 ? `${line.slice(0, 42)}…` : line
})

const title = computed(() => {
  if (resolved.value === 'deep') return props.streaming ? '深度思考中' : '深度思考'
  if (resolved.value === 'light') return props.streaming && !props.text ? '轻量思考中' : '轻量思考'
  return ''
})

const meta = computed(() => {
  if (seconds.value > 0) return `用时 ${seconds.value} 秒`
  if (props.streaming) return '正在展开'
  if (steps.value.length) return `${steps.value.length} 步`
  return ''
})

function stopTimer() {
  if (!timer) return
  window.clearInterval(timer)
  timer = 0
}

watch(() => props.streaming, (streaming) => {
  if (!touched) open.value = streaming || resolved.value === 'deep'
  if (streaming && resolved.value === 'deep') {
    if (!timer) {
      const start = Date.now()
      seconds.value = 0
      timer = window.setInterval(() => {
        seconds.value = Math.max(1, Math.round((Date.now() - start) / 1000))
      }, 400)
    }
    return
  }
  stopTimer()
}, { immediate: true })

function toggle() {
  touched = true
  open.value = !open.value
}

onUnmounted(stopTimer)
</script>

<template>
  <p v-if="visible && resolved === 'off'" class="thinking-off">{{ text }}</p>
  <div v-else-if="visible && resolved === 'light'" class="thinking-light">
    <button type="button" class="thinking-toggle" :aria-expanded="String(open)" @click="toggle">
      <i v-if="streaming" class="thinking-pulse" />
      <Icon v-else name="spark" :size="14" />
      <strong>{{ title }}</strong>
      <span v-if="!open" class="thinking-preview">{{ streaming && !text ? '思考中' : preview }}</span>
      <Icon name="chevron" :size="14" :class="{ open }" />
    </button>
    <p v-if="open && text">{{ text }}</p>
  </div>
  <section v-else-if="visible" class="thinking-deep" :class="{ live: streaming }">
    <button type="button" class="thinking-toggle" :aria-expanded="String(open)" @click="toggle">
      <i v-if="streaming" class="thinking-pulse" />
      <Icon v-else name="spark" :size="15" />
      <strong>{{ title }}</strong>
      <span v-if="meta" class="thinking-meta">{{ meta }}</span>
      <Icon name="chevron" :size="14" :class="{ open }" />
    </button>
    <ol v-if="open && steps.length" class="thinking-steps">
      <li v-for="(step, index) in steps" :key="index">{{ step }}</li>
    </ol>
    <p v-else-if="open && streaming" class="thinking-wait">正在整理步骤</p>
  </section>
</template>
