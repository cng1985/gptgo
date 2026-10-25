<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import TopBar from '../components/TopBar.vue'
import Icon from '../components/Icon.vue'
import { chat, resetDraft, stop } from '../stores/chat'

const features = [
  { path: '/agents', icon: 'bot', title: '智能体', text: '用一句话生成有自己设定的助手，确认后开始对话' },
  { path: '/skills', icon: 'code', title: '技能', text: '保存一段脚本，对话里按名称调用并执行' },
  { path: '/bi', icon: 'chart', title: '问数', text: '查询本库数据表，或连接自己的数据库' },
  { path: '/datasources', icon: 'database', title: '数据源', text: '配置 MySQL、PostgreSQL、SQL Server' },
  { path: '/billing', icon: 'coin', title: '账单', text: '余额、赠送额度和充值记录' },
]
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
      <div class="feature-grid">
        <button v-for="item in features" :key="item.path" type="button" @click="router.push(item.path)">
          <Icon :name="item.icon" :size="18" />
          <strong>{{ item.title }}</strong>
          <span>{{ item.text }}</span>
        </button>
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
