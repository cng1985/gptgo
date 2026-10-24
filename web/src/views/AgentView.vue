<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import PageHead from '../components/PageHead.vue'
import { models, notify } from '../stores/chat'
import { agents, loadAgent, openAgentChat, refreshAgents, removeAgent, saveAgent } from '../stores/agent'
import { mcp, refreshServers } from '../stores/mcp'

const router = useRouter()
const saving = ref(false)
const form = reactive(blank())

function blank() {
  return {
    id: '',
    name: '',
    description: '',
    instructions: '',
    model: 'builtin',
    opening: '',
    suggestions: ['', '', '', ''],
    tools: [],
    mcpIds: [],
  }
}

function fill(agent) {
  const suggestions = [...(agent.suggestions || [])]
  while (suggestions.length < 4) suggestions.push('')
  Object.assign(form, {
    id: agent.id,
    name: agent.name,
    description: agent.description || '',
    instructions: agent.instructions || '',
    model: agent.model || 'builtin',
    opening: agent.opening || '',
    suggestions: suggestions.slice(0, 4),
    tools: [...(agent.tools || [])],
    mcpIds: [...(agent.mcp_ids || [])].map((id) => String(id)),
  })
}

function reset() {
  Object.assign(form, blank())
}

async function reload() {
  try {
    await refreshAgents()
    await refreshServers()
  } catch (error) {
    notify(error.message || '加载失败')
  }
}

async function edit(item) {
  try {
    fill(await loadAgent(item.id))
  } catch (error) {
    notify(error.message || '读取失败')
  }
}

async function save() {
  saving.value = true
  try {
    const saved = await saveAgent({
      id: form.id,
      name: form.name.trim(),
      description: form.description.trim(),
      instructions: form.instructions.trim(),
      model: form.model,
      opening: form.opening.trim(),
      suggestions: form.suggestions.map((item) => item.trim()).filter(Boolean),
      tools: form.tools,
      mcpIds: form.mcpIds,
    })
    fill(saved)
    notify('已保存智能体')
  } catch (error) {
    notify(error.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function remove() {
  if (!form.id) return
  try {
    await removeAgent(form.id)
    reset()
    notify('已删除智能体')
  } catch (error) {
    notify(error.message || '删除失败')
  }
}

function talk() {
  if (!form.id) return
  openAgentChat(router, {
    id: form.id,
    model: form.model,
  })
}

onMounted(reload)
</script>

<template>
  <div class="panel-page">
    <PageHead title="智能体" />
    <div class="panel-body">
      <header class="panel-intro">
        <h1>按你的设定回答</h1>
        <p>每个智能体有自己的名称、开场白和回复规则。开始对话后，后续回答都会带上这段设定。勾选工具后，对话里可以调用当前时间和本库问数，也可以使用已启用的 MCP 服务。</p>
      </header>
      <div class="agent-layout">
        <section class="panel-card">
          <div class="section-head">
            <h2>我的智能体</h2>
            <button class="ghost" type="button" @click="reset">新建</button>
          </div>
          <button
            v-for="item in agents.items"
            :key="item.id"
            class="history-row"
            :class="{ selected: String(form.id) === String(item.id) }"
            type="button"
            @click="edit(item)"
          >
            <strong>{{ item.name }}</strong>
            <span>{{ item.description || '没有简介' }}</span>
          </button>
          <p v-if="!agents.items.length" class="muted">还没有智能体。</p>
        </section>
        <form class="panel-card source-form" @submit.prevent="save">
          <h2>{{ form.id ? '编辑智能体' : '新建智能体' }}</h2>
          <label class="field">
            名称
            <input v-model="form.name" maxlength="40" placeholder="例如：周报助手" />
          </label>
          <label class="field">
            简介
            <input v-model="form.description" maxlength="200" placeholder="一句话说明它做什么" />
          </label>
          <label class="field">
            模型
            <select v-model="form.model">
              <option v-for="item in models" :key="item.id" :value="item.id">{{ item.label }}</option>
            </select>
          </label>
          <label class="field">
            设定
            <textarea v-model="form.instructions" maxlength="4000" placeholder="例如：只用三句话回答，先给结论，再列依据。" />
          </label>
          <label class="field">
            开场白
            <textarea v-model="form.opening" maxlength="500" placeholder="用户打开对话时看到的第一句话" />
          </label>
          <div class="field">
            建议问题
            <input v-for="(_, index) in form.suggestions" :key="index" v-model="form.suggestions[index]" maxlength="80" placeholder="选填" />
          </div>
          <div class="field">
            工具
            <label class="check-line"><input v-model="form.tools" type="checkbox" value="clock" /> 当前时间</label>
            <label class="check-line"><input v-model="form.tools" type="checkbox" value="ask_catalog" /> 本库问数</label>
          </div>
          <div class="field">
            MCP
            <label v-for="item in mcp.items" :key="item.id" class="check-line">
              <input v-model="form.mcpIds" type="checkbox" :value="String(item.id)" />
              {{ item.name }}{{ item.enabled ? '' : '（未启用）' }}
            </label>
            <p v-if="!mcp.items.length" class="muted">还没有 MCP 服务。可以先到侧栏的 MCP 页面添加。</p>
          </div>
          <div class="form-actions">
            <button class="primary" type="submit" :disabled="saving">{{ saving ? '保存中' : '保存' }}</button>
            <button v-if="form.id" class="ghost" type="button" @click="talk">开始对话</button>
            <button v-if="form.id" class="ghost danger-text" type="button" @click="remove">删除</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
