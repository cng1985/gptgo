<script setup>
import { onMounted, reactive, ref } from 'vue'
import PageHead from '../components/PageHead.vue'
import { notify } from '../stores/chat'
import { loadServer, mcp, refreshServers, removeServer, saveServer, testServer } from '../stores/mcp'

const saving = ref(false)
const testing = ref(false)
const clearHeaders = ref(false)
const tested = ref([])
const form = reactive(blank())

function blank() {
  return {
    id: '',
    name: '',
    transport: 'http',
    url: '',
    command: '',
    argsText: '',
    headersText: '',
    headerKeys: [],
    headersSet: false,
    enabled: true,
  }
}

function fill(server) {
  Object.assign(form, {
    id: server.id,
    name: server.name,
    transport: server.transport || 'http',
    url: server.url || '',
    command: server.command || '',
    argsText: (server.args || []).join('\n'),
    headersText: '',
    headerKeys: server.header_keys || [],
    headersSet: !!server.headers_set,
    enabled: server.enabled !== false,
  })
  clearHeaders.value = false
  tested.value = []
}

function reset() {
  Object.assign(form, blank())
  clearHeaders.value = false
  tested.value = []
}

function args() {
  return form.argsText.split('\n').map((item) => item.trim()).filter(Boolean)
}

function headers() {
  if (clearHeaders.value) return {}
  const text = form.headersText.trim()
  if (!text) return null
  const result = {}
  for (const line of form.headersText.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed) continue
    const index = trimmed.indexOf(':')
    if (index <= 0) throw new Error('请求头需要写成 Key: Value')
    result[trimmed.slice(0, index).trim()] = trimmed.slice(index + 1).trim()
  }
  return result
}

function payload() {
  const body = {
    id: form.id,
    name: form.name.trim(),
    transport: form.transport,
    url: form.transport === 'http' ? form.url.trim() : '',
    command: form.transport === 'stdio' ? form.command.trim() : '',
    args: form.transport === 'stdio' ? args() : [],
    enabled: form.enabled,
  }
  const parsed = headers()
  if (parsed) body.headers = parsed
  return body
}

async function reload() {
  try {
    await refreshServers()
  } catch (error) {
    notify(error.message || '加载失败')
  }
}

async function edit(item) {
  try {
    fill(await loadServer(item.id))
  } catch (error) {
    notify(error.message || '读取失败')
  }
}

async function save() {
  saving.value = true
  try {
    fill(await saveServer(payload()))
    notify('已保存 MCP 服务')
  } catch (error) {
    notify(error.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function test() {
  testing.value = true
  tested.value = []
  try {
    const result = await testServer(payload())
    tested.value = result.tools || []
    notify(tested.value.length ? `读到 ${tested.value.length} 个工具` : '这个服务没有工具')
  } catch (error) {
    notify(error.message || '测试失败')
  } finally {
    testing.value = false
  }
}

async function remove() {
  if (!form.id) return
  try {
    await removeServer(form.id)
    reset()
    notify('已删除 MCP 服务')
  } catch (error) {
    notify(error.message || '删除失败')
  }
}

onMounted(reload)
</script>

<template>
  <div class="panel-page">
    <PageHead title="MCP" />
    <div class="panel-body">
      <header class="panel-intro">
        <h1>把外部工具交给智能体</h1>
        <p>用 HTTP 或本地命令接入 MCP 服务。请求头只在服务端加密保存，页面只显示名称。智能体勾选并启用后，对话里才会调用。</p>
      </header>
      <div class="agent-layout">
        <section class="panel-card">
          <div class="section-head">
            <h2>我的服务</h2>
            <button class="ghost" type="button" @click="reset">新建</button>
          </div>
          <button
            v-for="item in mcp.items"
            :key="item.id"
            class="history-row"
            :class="{ selected: String(form.id) === String(item.id) }"
            type="button"
            @click="edit(item)"
          >
            <strong>{{ item.name }}</strong>
            <span>{{ item.transport === 'stdio' ? '本地命令' : 'HTTP' }}{{ item.enabled ? '' : ' · 未启用' }}</span>
          </button>
          <p v-if="!mcp.items.length" class="muted">还没有 MCP 服务。</p>
        </section>
        <form class="panel-card source-form" @submit.prevent="save">
          <h2>{{ form.id ? '编辑服务' : '新建服务' }}</h2>
          <label class="field">
            名称
            <input v-model="form.name" maxlength="40" placeholder="例如：文档检索" />
          </label>
          <label class="field">
            传输方式
            <select v-model="form.transport">
              <option value="http">HTTP</option>
              <option value="stdio">本地命令</option>
            </select>
          </label>
          <label v-if="form.transport === 'http'" class="field">
            地址
            <input v-model="form.url" maxlength="500" placeholder="https://example.com/mcp" />
          </label>
          <template v-else>
            <label class="field">
              命令
              <input v-model="form.command" maxlength="200" placeholder="例如：npx" />
            </label>
            <label class="field">
              参数
              <textarea v-model="form.argsText" placeholder="每行一个参数" />
            </label>
          </template>
          <label class="field">
            请求头
            <textarea v-model="form.headersText" placeholder="每行一个，例如 Authorization: Bearer token。留空表示不修改。" />
          </label>
          <p v-if="form.headersSet" class="muted">已保存：{{ form.headerKeys.join('、') || '有请求头' }}</p>
          <label v-if="form.id && form.headersSet" class="check-line">
            <input v-model="clearHeaders" type="checkbox" />
            清空已保存的请求头
          </label>
          <label class="check-line">
            <input v-model="form.enabled" type="checkbox" />
            启用
          </label>
          <ul v-if="tested.length" class="tool-list">
            <li v-for="tool in tested" :key="tool.name" class="tool-trace">
              <strong>{{ tool.name }}</strong>
              <span v-if="tool.description"> {{ tool.description }}</span>
            </li>
          </ul>
          <div class="form-actions">
            <button class="primary" type="submit" :disabled="saving">{{ saving ? '保存中' : '保存' }}</button>
            <button class="ghost" type="button" :disabled="testing" @click="test">{{ testing ? '测试中' : '测试' }}</button>
            <button v-if="form.id" class="ghost danger-text" type="button" @click="remove">删除</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
