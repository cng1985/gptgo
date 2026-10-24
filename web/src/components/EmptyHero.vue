<script setup>
import { computed } from 'vue'
import SparkleMark from './SparkleMark.vue'
import { findAgent } from '../stores/agent'
import { chat, send } from '../stores/chat'

const fallback = [
  ['帮我规划', '制定一份清晰可执行的学习计划'],
  ['协助写作', '把想法整理成专业的文字'],
  ['分析问题', '从信息中提炼结论和行动建议'],
  ['解释概念', '用简单的例子讲清复杂问题'],
]

const agent = computed(() => findAgent(chat.agentId))
const cards = computed(() => {
  const suggestions = agent.value?.suggestions || []
  if (!suggestions.length) return fallback
  return suggestions.map((text) => [text, ''])
})

function useCard(title, detail) {
  send({ content: detail ? `${title}：${detail}` : title })
}
</script>

<template>
  <div class="hero">
    <div class="hero-mark"><SparkleMark :size="30" /></div>
    <h1>{{ agent ? agent.name : '有什么可以帮忙的?' }}</h1>
    <p>{{ agent ? (agent.opening || agent.description || '按这个智能体的设定开始对话。') : '我可以帮你分析、创作、编程和解决问题。' }}</p>
    <div class="cards">
      <button v-for="card in cards" :key="card[0]" type="button" @click="useCard(card[0], card[1])">
        <strong>{{ card[0] }}</strong>
        <span v-if="card[1]">{{ card[1] }}</span>
      </button>
    </div>
  </div>
</template>
