<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import PageHead from '../components/PageHead.vue'
import { models, notify } from '../stores/chat'
import { agents, draftAgent, loadAgent, openAgentChat, refreshAgents, removeAgent, saveAgent } from '../stores/agent'

const router = useRouter()
const saving = ref(false)
const drafting = ref(false)
const idea = ref('')
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
  })
}

function reset() {
  Object.assign(form, blank())
  idea.value = ''
}

async function reload() {
  try {
    await refreshAgents()
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

async function generate() {
  const prompt = idea.value.trim()
  if (!prompt) {
    notify('先描述你想要的智能体')
    return
  }
  drafting.value = true
  try {
    const draft = await draftAgent({ prompt, model: form.model })
    fill({ ...draft, id: form.id })
    notify('已生成草稿，确认后保存')
  } catch (error) {
    notify(error.message || '生成失败')
  } finally {
    drafting.value = false
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
        <p>每个智能体有自己的名称、开场白和回复规则。用一句话生成草稿，确认后再保存。开始对话后，后续回答都会带上这段设定。问数仍然在单独的页面里。</p>
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
            模型
            <select v-model="form.model">
              <option v-for="item in models" :key="item.id" :value="item.id">{{ item.label }}</option>
            </select>
          </label>
          <div class="draft-box">
            <label class="field">
              用一句话创建
              <textarea v-model="idea" maxlength="1000" placeholder="例如：做一个周报助手，只用三句话，先给结论" />
            </label>
            <div class="form-actions">
              <button class="opt-chip" type="button" :disabled="drafting || !idea.trim()" @click="generate">
                {{ drafting ? '生成中' : 'AI 生成' }}
              </button>
            </div>
            <p class="form-note">按上方选择的模型生成，填进表单后才保存。选内置助手或没有密钥时在本地生成。描述里写明 DeepSeek 或 MiniMax 时，会改用对应模型。</p>
          </div>
          <label class="field">
            名称
            <input v-model="form.name" maxlength="40" placeholder="例如：周报助手" />
          </label>
          <label class="field">
            简介
            <input v-model="form.description" maxlength="200" placeholder="一句话说明它做什么" />
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
