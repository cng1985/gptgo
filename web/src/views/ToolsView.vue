<script setup>
import { onMounted, reactive, ref } from 'vue'
import PageHead from '../components/PageHead.vue'
import { notify } from '../stores/chat'
import { loadHTTP, loadMCP, refreshToolkit, removeHTTP, removeMCP, saveHTTP, saveMCP, setBuiltin, testMCP, toolkit } from '../stores/toolkit'

const savingHTTP = ref(false)
const savingMCP = ref(false)
const testing = ref(false)
const httpForm = reactive(blankHTTP())
const mcpForm = reactive(blankMCP())

function blankHTTP() {
  return {
    id: '',
    name: '',
    description: '',
    parameters: '',
    method: 'GET',
    url: '',
    headers: '',
    enabled: true,
  }
}

function blankMCP() {
  return {
    id: '',
    name: '',
    transport: 'stdio',
    command: '',
    argsText: '',
    env: '',
    url: '',
    headers: '',
    enabled: true,
  }
}

function fillHTTP(item) {
  Object.assign(httpForm, {
    id: item.id,
    name: item.name,
    description: item.description || '',
    parameters: item.parameters || '',
    method: item.method || 'GET',
    url: item.url || '',
    headers: '',
    enabled: !!item.enabled,
  })
}

function fillMCP(item) {
  Object.assign(mcpForm, {
    id: item.id,
    name: item.name,
    transport: item.transport || 'stdio',
    command: item.command || '',
    argsText: (item.args || []).join('\n'),
    env: '',
    url: item.url || '',
    headers: '',
    enabled: !!item.enabled,
  })
}

async function reload() {
  try {
    await refreshToolkit()
  } catch (error) {
    notify(error.message || '加载失败')
  }
}

async function editHTTP(item) {
  try {
    fillHTTP(await loadHTTP(item.id))
  } catch (error) {
    notify(error.message || '读取失败')
  }
}

async function editMCP(item) {
  toolkit.remote = []
  try {
    fillMCP(await loadMCP(item.id))
  } catch (error) {
    notify(error.message || '读取失败')
  }
}

async function toggleBuiltin(item) {
  try {
    await setBuiltin(item.name, !item.enabled)
  } catch (error) {
    notify(error.message || '更新失败')
  }
}

async function submitHTTP() {
  savingHTTP.value = true
  try {
    const saved = await saveHTTP({
      id: httpForm.id,
      name: httpForm.name.trim(),
      description: httpForm.description.trim(),
      parameters: httpForm.parameters.trim(),
      method: httpForm.method,
      url: httpForm.url.trim(),
      headers: httpForm.headers.trim(),
      enabled: httpForm.enabled,
    })
    fillHTTP(saved)
    notify('已保存 HTTP 工具')
  } catch (error) {
    notify(error.message || '保存失败')
  } finally {
    savingHTTP.value = false
  }
}

async function submitMCP() {
  savingMCP.value = true
  try {
    const saved = await saveMCP({
      id: mcpForm.id,
      name: mcpForm.name.trim(),
      transport: mcpForm.transport,
      command: mcpForm.command.trim(),
      args: mcpForm.argsText.split('\n').map((item) => item.trim()).filter(Boolean),
      env: mcpForm.env.trim(),
      url: mcpForm.url.trim(),
      headers: mcpForm.headers.trim(),
      enabled: mcpForm.enabled,
    })
    fillMCP(saved)
    notify('已保存 MCP 服务')
  } catch (error) {
    notify(error.message || '保存失败')
  } finally {
    savingMCP.value = false
  }
}

async function deleteHTTP() {
  if (!httpForm.id) return
  try {
    await removeHTTP(httpForm.id)
    Object.assign(httpForm, blankHTTP())
    notify('已删除 HTTP 工具')
  } catch (error) {
    notify(error.message || '删除失败')
  }
}

async function deleteMCP() {
  if (!mcpForm.id) return
  try {
    await removeMCP(mcpForm.id)
    Object.assign(mcpForm, blankMCP())
    notify('已删除 MCP 服务')
  } catch (error) {
    notify(error.message || '删除失败')
  }
}

async function probeMCP() {
  if (!mcpForm.id) return
  testing.value = true
  toolkit.remote = []
  try {
    const tools = await testMCP(mcpForm.id)
    notify(tools.length ? `读到 ${tools.length} 个工具` : '这个服务没有工具')
  } catch (error) {
    notify(error.message || '连接失败')
  } finally {
    testing.value = false
  }
}

onMounted(reload)
</script>

<template>
  <div class="panel-page">
    <PageHead title="工具与 MCP" />
    <div class="panel-body">
      <header class="panel-intro">
        <h1>让回答能调用工具</h1>
        <p>内置工具默认开启。HTTP 工具按你写的地址发请求。MCP 可以用本机命令或 HTTP 服务，把对方的工具交给当前对话。密钥只加密存在本机，页面不再显示原文。</p>
      </header>

      <section class="panel-card tool-block">
        <h2>内置工具</h2>
        <div v-for="item in toolkit.builtins" :key="item.name" class="toggle-row">
          <div>
            <strong>{{ item.name }}</strong>
            <span>{{ item.description }}</span>
          </div>
          <button type="button" :class="item.enabled ? 'primary tiny' : 'ghost'" @click="toggleBuiltin(item)">
            {{ item.enabled ? '已开启' : '已关闭' }}
          </button>
        </div>
      </section>

      <div class="tool-layout">
        <section class="panel-card">
          <div class="section-head">
            <h2>HTTP 工具</h2>
            <button class="ghost" type="button" @click="Object.assign(httpForm, blankHTTP())">新建</button>
          </div>
          <button
            v-for="item in toolkit.https"
            :key="item.id"
            class="history-row"
            :class="{ selected: String(httpForm.id) === String(item.id) }"
            type="button"
            @click="editHTTP(item)"
          >
            <strong>{{ item.name }}</strong>
            <span>{{ item.method }} {{ item.url }}</span>
          </button>
          <p v-if="!toolkit.https.length" class="muted">还没有 HTTP 工具。</p>
        </section>
        <form class="panel-card source-form" @submit.prevent="submitHTTP">
          <h2>{{ httpForm.id ? '编辑 HTTP 工具' : '新建 HTTP 工具' }}</h2>
          <label class="field">名称<input v-model="httpForm.name" maxlength="40" placeholder="weather" /></label>
          <label class="field">说明<input v-model="httpForm.description" maxlength="300" placeholder="查询某个城市的天气" /></label>
          <div class="field-row">
            <label class="field">地址<input v-model="httpForm.url" maxlength="500" placeholder="https://example.com/weather" /></label>
            <label class="field">方法
              <select v-model="httpForm.method">
                <option value="GET">GET</option>
                <option value="POST">POST</option>
              </select>
            </label>
          </div>
          <label class="field">参数说明<textarea v-model="httpForm.parameters" placeholder='留空使用默认 JSON Schema' /></label>
          <label class="field">请求头<textarea v-model="httpForm.headers" placeholder='{"Authorization":"Bearer ..."}，编辑时留空表示不修改' /></label>
          <label class="check-row"><input v-model="httpForm.enabled" type="checkbox" /> 启用</label>
          <div class="form-actions">
            <button class="primary tiny" type="submit" :disabled="savingHTTP">{{ savingHTTP ? '保存中' : '保存' }}</button>
            <button v-if="httpForm.id" class="ghost danger-text" type="button" @click="deleteHTTP">删除</button>
          </div>
        </form>
      </div>

      <div class="tool-layout">
        <section class="panel-card">
          <div class="section-head">
            <h2>MCP 服务</h2>
            <button class="ghost" type="button" @click="Object.assign(mcpForm, blankMCP()); toolkit.remote = []">新建</button>
          </div>
          <button
            v-for="item in toolkit.servers"
            :key="item.id"
            class="history-row"
            :class="{ selected: String(mcpForm.id) === String(item.id) }"
            type="button"
            @click="editMCP(item)"
          >
            <strong>{{ item.name }}</strong>
            <span>{{ item.transport }} {{ item.transport === 'http' ? item.url : '命令' }}</span>
          </button>
          <p v-if="!toolkit.servers.length" class="muted">还没有 MCP 服务。</p>
        </section>
        <form class="panel-card source-form" @submit.prevent="submitMCP">
          <h2>{{ mcpForm.id ? '编辑 MCP 服务' : '新建 MCP 服务' }}</h2>
          <label class="field">名称<input v-model="mcpForm.name" maxlength="40" placeholder="例如：本地文件" /></label>
          <label class="field">传输
            <select v-model="mcpForm.transport">
              <option value="stdio">stdio 命令</option>
              <option value="http">HTTP</option>
            </select>
          </label>
          <template v-if="mcpForm.transport === 'stdio'">
            <label class="field">命令<input v-model="mcpForm.command" maxlength="200" placeholder="/usr/bin/my-mcp" /></label>
            <label class="field">参数<textarea v-model="mcpForm.argsText" placeholder="一行一个参数，不经过 shell" /></label>
            <label class="field">环境变量<textarea v-model="mcpForm.env" placeholder='{"API_KEY":"..."}，编辑时留空表示不修改' /></label>
          </template>
          <template v-else>
            <label class="field">地址<input v-model="mcpForm.url" maxlength="500" placeholder="http://127.0.0.1:3000/mcp" /></label>
            <label class="field">请求头<textarea v-model="mcpForm.headers" placeholder='{"Authorization":"Bearer ..."}，编辑时留空表示不修改' /></label>
          </template>
          <label class="check-row"><input v-model="mcpForm.enabled" type="checkbox" /> 启用</label>
          <div class="form-actions">
            <button class="primary tiny" type="submit" :disabled="savingMCP">{{ savingMCP ? '保存中' : '保存' }}</button>
            <button v-if="mcpForm.id" class="ghost" type="button" :disabled="testing" @click="probeMCP">{{ testing ? '连接中' : '测试连接' }}</button>
            <button v-if="mcpForm.id" class="ghost danger-text" type="button" @click="deleteMCP">删除</button>
          </div>
          <ul v-if="toolkit.remote.length" class="remote-tools">
            <li v-for="tool in toolkit.remote" :key="tool.name">
              <strong>{{ tool.name }}</strong>
              <span>{{ tool.description || '没有说明' }}</span>
            </li>
          </ul>
        </form>
      </div>
    </div>
  </div>
</template>
