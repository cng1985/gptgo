<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import PageHead from '../components/PageHead.vue'
import { notify } from '../stores/chat'
import { defaultParameters, loadSkill, refreshSkills, removeSkill, runSkill, saveSkill, skillSamples, skills } from '../stores/skill'

const runtimes = [
  { id: 'python', label: 'Python' },
  { id: 'node', label: 'Node.js' },
  { id: 'bash', label: 'Bash' },
]
const runtimeLabel = Object.fromEntries(runtimes.map((item) => [item.id, item.label]))
const saving = ref(false)
const running = ref(false)
const keyword = ref('')
const nameTouched = ref(false)
const runInput = ref('{\n  "input": "hello"\n}')
const runResult = ref(null)
const form = reactive(blank())

function blank() {
  return {
    id: '',
    title: '',
    name: '',
    description: '',
    instructions: '',
    runtime: 'python',
    script: skillSamples.python,
    parameters: defaultParameters,
    timeout_sec: 10,
    enabled: true,
  }
}

function pretty(raw) {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw || defaultParameters
  }
}

function fill(skill, options = {}) {
  nameTouched.value = true
  if (!options.keepRun) runResult.value = null
  Object.assign(form, {
    id: skill.id,
    title: skill.title || '',
    name: skill.name || '',
    description: skill.description || '',
    instructions: skill.instructions || '',
    runtime: skill.runtime || 'python',
    script: skill.script || '',
    parameters: pretty(skill.parameters),
    timeout_sec: skill.timeout_sec || 10,
    enabled: !!skill.enabled,
  })
}

function reset() {
  nameTouched.value = false
  runResult.value = null
  Object.assign(form, blank())
}

function suggestName(title) {
  const slug = title.trim().toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 32)
  return /^[a-z]/.test(slug) ? slug : ''
}

function onTitle(event) {
  form.title = event.target.value
  if (!nameTouched.value) {
    const next = suggestName(form.title)
    if (next) form.name = next
  }
}

function onName(event) {
  nameTouched.value = true
  form.name = event.target.value
}

function setRuntime(runtime) {
  const previous = skillSamples[form.runtime]
  if (!form.script.trim() || form.script === previous) form.script = skillSamples[runtime]
  form.runtime = runtime
}

function useSample() {
  form.script = skillSamples[form.runtime]
}

function onScriptTab(event) {
  if (event.key !== 'Tab') return
  event.preventDefault()
  const el = event.target
  const start = el.selectionStart
  const end = el.selectionEnd
  form.script = `${form.script.slice(0, start)}  ${form.script.slice(end)}`
  requestAnimationFrame(() => {
    el.selectionStart = el.selectionEnd = start + 2
  })
}

const visible = computed(() => {
  const query = keyword.value.trim().toLowerCase()
  if (!query) return skills.items
  return skills.items.filter((item) => `${item.title} ${item.name} ${item.description}`.toLowerCase().includes(query))
})

async function reload() {
  try {
    await refreshSkills()
  } catch (error) {
    notify(error.message || '加载失败')
  }
}

async function edit(item) {
  try {
    fill(await loadSkill(item.id))
  } catch (error) {
    notify(error.message || '读取失败')
  }
}

async function toggle(item) {
  try {
    const full = await loadSkill(item.id)
    const saved = await saveSkill({ ...full, enabled: !full.enabled })
    if (String(form.id) === String(item.id)) fill(saved, { keepRun: true })
  } catch (error) {
    notify(error.message || '更新失败')
  }
}

async function save() {
  saving.value = true
  try {
    const saved = await saveSkill({
      id: form.id,
      title: form.title.trim(),
      name: form.name.trim(),
      description: form.description.trim(),
      instructions: form.instructions.trim(),
      runtime: form.runtime,
      script: form.script,
      parameters: form.parameters.trim(),
      timeout_sec: Number(form.timeout_sec) || 10,
      enabled: form.enabled,
    })
    fill(saved, { keepRun: true })
    notify('已保存技能')
  } catch (error) {
    notify(error.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function remove() {
  if (!form.id) return
  try {
    await removeSkill(form.id)
    reset()
    notify('已删除技能')
  } catch (error) {
    notify(error.message || '删除失败')
  }
}

async function run() {
  if (!form.script.trim()) {
    notify('先写一段脚本')
    return
  }
  running.value = true
  runResult.value = null
  try {
    runResult.value = await runSkill({
      name: form.name.trim(),
      runtime: form.runtime,
      script: form.script,
      input: runInput.value,
      timeout_sec: Number(form.timeout_sec) || 10,
    })
  } catch (error) {
    notify(error.message || '运行失败')
  } finally {
    running.value = false
  }
}

function startFromSample() {
  reset()
  form.title = '转成大写'
  form.name = 'upper_text'
  form.description = '把 input 转成大写后返回'
  nameTouched.value = true
}

onMounted(reload)
</script>

<template>
  <div class="panel-page">
    <PageHead title="技能" />
    <div class="panel-body skill-body">
      <header class="panel-intro">
        <h1>把一段脚本变成技能</h1>
        <p>保存后可以在对话里调用。模型只传入 JSON，脚本从标准输入读取，标准输出作为结果。参数不会拼进命令行，应用密钥也不会传给脚本。</p>
      </header>
      <div class="skill-studio">
        <section class="panel-card skill-rail">
          <div class="section-head">
            <h2>我的技能</h2>
            <button class="ghost" type="button" @click="reset">新建</button>
          </div>
          <label class="skill-search">
            <input v-model="keyword" placeholder="搜索名称或调用名" />
          </label>
          <div v-if="visible.length" class="skill-list">
            <article
              v-for="item in visible"
              :key="item.id"
              class="skill-card"
              :class="{ selected: String(form.id) === String(item.id) }"
            >
              <button class="skill-card-main" type="button" @click="edit(item)">
                <span class="skill-card-top">
                  <strong>{{ item.title }}</strong>
                  <em>{{ runtimeLabel[item.runtime] || item.runtime }}</em>
                </span>
                <code>{{ item.name }}</code>
                <span>{{ item.description || '没有说明' }}</span>
              </button>
              <button class="skill-switch" type="button" @click="toggle(item)">
                <i :class="{ on: item.enabled }" />
                {{ item.enabled ? '已启用' : '已停用' }}
              </button>
            </article>
          </div>
          <div v-else class="skill-empty">
            <p>{{ keyword.trim() ? '没有匹配的技能' : '还没有技能。可以从一段示例脚本开始。' }}</p>
            <button v-if="!keyword.trim()" class="ghost" type="button" @click="startFromSample">用示例开始</button>
          </div>
        </section>

        <form class="panel-card skill-editor" @submit.prevent="save">
          <div class="skill-editor-head">
            <div>
              <h2>{{ form.id ? '编辑技能' : '新建技能' }}</h2>
              <p>对话里写「调用 {{ form.name || '调用名' }}」时，内置助手会执行这段脚本。</p>
            </div>
            <button class="skill-switch editor" type="button" @click="form.enabled = !form.enabled">
              <i :class="{ on: form.enabled }" />
              {{ form.enabled ? '已启用' : '已停用' }}
            </button>
          </div>

          <div class="segment" role="tablist" aria-label="运行环境">
            <button
              v-for="item in runtimes"
              :key="item.id"
              type="button"
              :class="{ active: form.runtime === item.id }"
              @click="setRuntime(item.id)"
            >
              {{ item.label }}
            </button>
          </div>

          <div class="skill-fields">
            <label class="field">
              显示名称
              <input :value="form.title" maxlength="40" placeholder="例如：转成大写" @input="onTitle" />
            </label>
            <label class="field">
              调用名
              <input :value="form.name" maxlength="32" placeholder="upper_text" spellcheck="false" @input="onName" />
            </label>
          </div>
          <label class="field">
            什么时候调用
            <input v-model="form.description" maxlength="300" placeholder="例如：用户要求把文字转成大写时使用" />
          </label>
          <label class="field">
            补充说明
            <textarea v-model="form.instructions" maxlength="2000" placeholder="选填。告诉模型参数里有哪些字段、结果是什么意思。" />
          </label>

          <div class="skill-work">
            <div>
              <div class="script-shell">
                <div class="script-bar">
                  <span>{{ runtimeLabel[form.runtime] }} · 标准输入是 JSON</span>
                  <div class="script-bar-actions">
                    <label>
                      超时
                      <select v-model.number="form.timeout_sec">
                        <option :value="5">5 秒</option>
                        <option :value="10">10 秒</option>
                        <option :value="20">20 秒</option>
                      </select>
                    </label>
                    <button type="button" @click="useSample">示例脚本</button>
                  </div>
                </div>
                <textarea
                  v-model="form.script"
                  spellcheck="false"
                  placeholder="在这里写脚本"
                  @keydown="onScriptTab"
                />
              </div>
              <details class="skill-advanced">
                <summary>参数结构</summary>
                <textarea v-model="form.parameters" spellcheck="false" />
                <p>留空会使用默认结构：一个名为 input 的字符串。模型按这个结构填参数。</p>
              </details>
              <div class="form-actions">
                <button class="primary" type="submit" :disabled="saving">{{ saving ? '保存中' : '保存' }}</button>
                <button v-if="form.id" class="ghost danger-text" type="button" @click="remove">删除</button>
              </div>
            </div>

            <section class="skill-console">
              <div class="skill-console-head">
                <h3>试运行</h3>
                <button class="ghost" type="button" :disabled="running || !form.script.trim()" @click="run">
                  {{ running ? '运行中' : '运行当前脚本' }}
                </button>
              </div>
              <p>运行编辑器里的当前脚本，不必先保存。结果不会写入对话。</p>
              <label class="field">
                输入 JSON
                <textarea v-model="runInput" class="json-input" spellcheck="false" />
              </label>
              <div v-if="runResult" class="run-grid">
                <div class="run-status">
                  <span class="pill" :class="runResult.ok ? 'ok' : 'error'">{{ runResult.ok ? '成功' : '失败' }}</span>
                  <span>退出码 {{ runResult.exit_code }}</span>
                  <span>{{ runResult.duration_ms }} 毫秒</span>
                </div>
                <article>
                  <h4>标准输出</h4>
                  <pre>{{ runResult.stdout || '没有标准输出' }}</pre>
                </article>
                <article>
                  <h4>标准错误</h4>
                  <pre>{{ runResult.stderr || '没有标准错误' }}</pre>
                </article>
              </div>
              <p v-else class="run-placeholder">运行后，标准输出会显示在这里。</p>
            </section>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
