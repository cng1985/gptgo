<script setup>
import SparkleMark from './SparkleMark.vue'
import { send } from '../stores/chat'

const cards = [
  ['帮我规划', '制定一份清晰可执行的学习计划'],
  ['协助写作', '把想法整理成专业的文字'],
  ['分析数据', '从信息中提炼结论和行动建议'],
  ['解释概念', '用简单的例子讲清复杂问题'],
]

function useCard(title, detail) {
  send({ content: `${title}：${detail}` })
}
</script>

<template>
  <div class="hero">
    <div class="hero-mark"><SparkleMark :size="30" /></div>
    <h1>有什么可以帮忙的?</h1>
    <p>我可以帮你分析、创作、编程和解决问题。</p>
    <div class="cards">
      <button v-for="card in cards" :key="card[0]" type="button" @click="useCard(card[0], card[1])">
        <strong>{{ card[0] }}</strong>
        <span>{{ card[1] }}</span>
      </button>
    </div>
  </div>
</template>
