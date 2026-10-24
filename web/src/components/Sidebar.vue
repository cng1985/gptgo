<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Icon from './Icon.vue'
import SparkleMark from './SparkleMark.vue'
import { chat, logout, refreshConversations, removeConversation, renameConversation, resetDraft, saveName, stop } from '../stores/chat'
import { groupLabel } from '../utils/time'

const productLinks = [
  { name: 'bi', path: '/bi', icon: 'chart', label: '问数' },
  { name: 'datasources', path: '/datasources', icon: 'database', label: '数据源' },
  { name: 'usage', path: '/usage', icon: 'spark', label: '用量' },
  { name: 'billing', path: '/billing', icon: 'coin', label: '账单' },
]

const route = useRoute()
const router = useRouter()
const searchOpen = ref(false)
const searchRef = ref(null)
const menuId = ref('')
const renamingId = ref('')
const renameText = ref('')
const profileOpen = ref(false)
const nameDraft = ref('')
let searchTimer = 0

function eventTarget(event) {
  if (event.target instanceof Element) return event.target
  return event.target?.parentElement || null
}

function onPointerDown(event) {
  const target = eventTarget(event)
  if (target?.closest('.history-item') || target?.closest('.user-row') || target?.closest('.menu')) return
  menuId.value = ''
  profileOpen.value = false
}

onMounted(() => {
  document.addEventListener('pointerdown', onPointerDown, true)
  document.addEventListener('click', onPointerDown, true)
})
onUnmounted(() => {
  document.removeEventListener('pointerdown', onPointerDown, true)
  document.removeEventListener('click', onPointerDown, true)
})

const groups = computed(() => {
  const order = ['今天', '昨天', '最近 7 天', '更早']
  const buckets = Object.fromEntries(order.map((label) => [label, []]))
  for (const item of chat.conversations) {
    buckets[groupLabel(item.updated_at)].push(item)
  }
  return order.filter((label) => buckets[label].length).map((label) => ({ label, items: buckets[label] }))
})

function openPath(path) {
  menuId.value = ''
  chat.sidebarOpen = false
  router.push(path)
}

function newChat() {
  stop()
  resetDraft()
  menuId.value = ''
  chat.sidebarOpen = false
  router.push('/')
}

function openSearch() {
  searchOpen.value = true
  requestAnimationFrame(() => searchRef.value?.focus())
}

function onSearchInput(event) {
  chat.keyword = event.target.value
  window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    refreshConversations().catch((error) => {
      chat.toast = error.message
    })
  }, 180)
}

function openItem(id) {
  if (chat.streaming) stop()
  menuId.value = ''
  chat.sidebarOpen = false
  router.push(`/c/${id}`)
}

function startRename(item) {
  renamingId.value = String(item.id)
  renameText.value = item.title
  menuId.value = ''
}

async function commitRename(item) {
  const title = renameText.value.trim()
  renamingId.value = ''
  if (!title || title === item.title) return
  try {
    await renameConversation(item.id, title)
  } catch (error) {
    chat.toast = error.message
  }
}

async function remove(item) {
  menuId.value = ''
  try {
    await removeConversation(item.id)
    if (route.params.id && String(route.params.id) === String(item.id)) router.push('/')
  } catch (error) {
    chat.toast = error.message
  }
}

function openProfile() {
  profileOpen.value = !profileOpen.value
  nameDraft.value = chat.account.name || 'ada'
}

async function saveProfile() {
  try {
    await saveName(nameDraft.value.trim())
    profileOpen.value = false
  } catch (error) {
    chat.toast = error.message
  }
}

async function signOut() {
  profileOpen.value = false
  try {
    await logout()
    router.push('/login')
  } catch (error) {
    chat.toast = error.message
  }
}
</script>

<template>
  <aside class="sidebar">
    <div class="brand">
      <span class="brand-mark"><SparkleMark :size="16" /></span>
      <span>ChatBI AI</span>
    </div>
    <button class="new-chat" type="button" @click="newChat">
      <span class="new-chat-label"><Icon name="pencil" :size="16" /> 新建对话</span>
      <span class="kbd">Ctrl K</span>
    </button>
    <nav class="side-nav">
      <button
        v-for="item in productLinks"
        :key="item.name"
        type="button"
        :class="{ active: route.name === item.name }"
        @click="openPath(item.path)"
      >
        <Icon :name="item.icon" :size="15" />
        <span>{{ item.label }}</span>
      </button>
    </nav>
    <button v-if="!searchOpen" class="search-entry" type="button" @click="openSearch">
      <Icon name="search" :size="16" />
      <span>搜索对话</span>
    </button>
    <label v-else class="search-box">
      <Icon name="search" :size="16" />
      <input ref="searchRef" :value="chat.keyword" placeholder="搜索对话" @input="onSearchInput" />
    </label>
    <div class="history">
      <p v-if="chat.booted && !groups.length" class="history-empty">没有匹配的对话</p>
      <section v-for="group in groups" :key="group.label" class="history-group">
        <h3>{{ group.label }}</h3>
        <div
          v-for="item in group.items"
          :key="item.id"
          class="history-item"
          :class="{ active: String(route.params.id) === String(item.id) }"
        >
          <input
            v-if="renamingId === String(item.id)"
            v-model="renameText"
            class="rename-input"
            @keydown.enter="commitRename(item)"
            @blur="commitRename(item)"
          />
          <button v-else class="history-title" type="button" :title="item.title" @click="openItem(item.id)">
            {{ item.title }}
          </button>
          <button class="more" type="button" aria-label="对话操作" @click.stop="menuId = menuId === String(item.id) ? '' : String(item.id)">
            <Icon name="more" :size="16" />
          </button>
          <div v-if="menuId === String(item.id)" class="menu item-menu">
            <button type="button" @click="startRename(item)">重命名</button>
            <button type="button" class="danger" @click="remove(item)">删除</button>
          </div>
        </div>
      </section>
    </div>
    <div class="sidebar-foot">
      <button class="workspace-link" :class="{ active: route.name === 'workspace' }" type="button" @click="router.push('/workspace'); chat.sidebarOpen = false">
        <Icon name="home" :size="16" />
        <span>返回工作台</span>
      </button>
      <div class="user-row">
        <button class="user-button" type="button" @click="openProfile">
          <span class="avatar">{{ (chat.account.name || chat.account.username || 'A').slice(0, 1).toUpperCase() }}</span>
          <span class="user-copy">
            <strong>{{ chat.account.name || chat.account.username || 'ada' }}</strong>
            <small>{{ chat.account.username || '个人中心' }}</small>
          </span>
        </button>
        <button class="more always" type="button" aria-label="个人中心" @click="openProfile">
          <Icon name="more" :size="16" />
        </button>
        <div v-if="profileOpen" class="menu profile-menu">
          <label>
            显示名称
            <input v-model="nameDraft" maxlength="20" @keydown.enter.prevent="saveProfile" />
          </label>
          <button type="button" @click="saveProfile">保存</button>
          <button v-if="chat.account.username" type="button" class="danger" @click="signOut">退出登录</button>
          <p class="about">登录后只显示这个账号的对话。模型密钥通过环境变量接入。</p>
        </div>
      </div>
    </div>
  </aside>
</template>
