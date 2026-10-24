<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import ChartPanel from '../components/ChartPanel.vue'
import PageHead from '../components/PageHead.vue'
import { chat, models, notify } from '../stores/chat'
import { askCatalog, askQuestion, chatbi, loadSchema, refreshBalance, refreshSources, searchUsage } from '../stores/chatbi'
import { renderMarkdown } from '../utils/markdown'
import { formatDateTime } from '../utils/time'

const router = useRouter()
const datasourceId = ref(localStorage.getItem('chatbi-datasource') || '')
const model = ref(localStorage.getItem('chatbi-model') || 'builtin')
const question = ref('')
const asking = ref(false)
const result = ref(null)
const errorText = ref('')
const tables = ref([])
const history = ref([])
const catalogQuestion = ref('读取系统全部数据表，并用图表展示')
const catalogAsking = ref(false)
const catalogResult = ref(null)
const catalogError = ref('')
const catalogIdeas = [
  '读取系统全部数据表，并用图表展示',
  '对话按模型画饼图',
  'account 有哪些字段',
]
const catalogHtml = computed(() => renderMarkdown(catalogResult.value?.answer || ''))

const source = computed(() => chatbi.sources.find((item) => String(item.id) === String(datasourceId.value)))
const suggestions = computed(() => {
  const items = []
  const first = tables.value[0]
  if (!first) return ['有哪些表']
  items.push(`${first.name} 有多少条`)
  const text = first.columns?.find((column) => /char|text|string/i.test(column.data_type) && column.name.toLowerCase() !== 'id')
  const number = first.columns?.find((column) => /int|decimal|numeric|float|double|money/i.test(column.data_type) && !/_?id$/i.test(column.name))
  if (text) items.push(`每个 ${text.name} 的 ${first.name} 数量`)
  if (number) items.push(`${first.name} 的 ${number.name} 总额`)
  items.push('有哪些表')
  return items.slice(0, 4)
})
const bars = computed(() => chartItems(result.value))
const linePoints = computed(() => {
  const items = bars.value
  if (!items.length) return ''
  return items.map((item, index) => {
    const x = items.length === 1 ? 160 : (index / (items.length - 1)) * 300 + 10
    const y = 110 - (item.value / item.max) * 90
    return `${x},${y}`
  }).join(' ')
})

function chartItems(payload) {
  if (!payload || (payload.chart !== 'bar' && payload.chart !== 'line')) return []
  const last = (payload.columns || []).length - 1
  const items = (payload.rows || []).map((row) => ({
    label: String(row?.[0] ?? ''),
    value: asNumber(row?.[last]),
  })).filter((item) => item.value != null).slice(0, 12)
  const max = Math.max(...items.map((item) => Math.abs(item.value)), 1)
  return items.map((item) => ({ ...item, max, width: Math.max(6, (Math.abs(item.value) / max) * 100) }))
}

function asNumber(value) {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '' && Number.isFinite(Number(value))) return Number(value)
  return null
}

function cellText(value) {
  if (value == null) return ''
  return String(value)
}

async function boot() {
  try {
    await Promise.all([refreshSources(), refreshBalance(), loadHistory()])
    if (!chatbi.sources.some((item) => String(item.id) === String(datasourceId.value))) {
      datasourceId.value = chatbi.sources[0]?.id || ''
    }
    await loadTables()
  } catch (error) {
    notify(error.message || '加载失败')
  }
}

async function loadHistory() {
  const page = await searchUsage({ page: 1, pageSize: 8 })
  history.value = page.data || []
}

async function loadTables() {
  tables.value = []
  if (!datasourceId.value) return
  try {
    const schema = await loadSchema(datasourceId.value)
    tables.value = schema.data.tables || []
  } catch {
    tables.value = []
  }
}

async function submitCatalog() {
  const text = catalogQuestion.value.trim()
  if (!text || catalogAsking.value) return
  catalogAsking.value = true
  catalogError.value = ''
  try {
    const response = await askCatalog(text, chat.thinking)
    catalogResult.value = response.data
  } catch (error) {
    catalogError.value = error.message || '问数失败'
  } finally {
    catalogAsking.value = false
  }
}

async function submit() {
  const text = question.value.trim()
  if (!datasourceId.value) {
    errorText.value = '先配置一个数据源'
    return
  }
  if (!text || asking.value) return
  asking.value = true
  errorText.value = ''
  try {
    const response = await askQuestion({ datasourceId: datasourceId.value, question: text, model: model.value })
    result.value = response.data
    chatbi.balance = response.data.balance
    question.value = ''
    await loadHistory()
  } catch (error) {
    errorText.value = error.message || '查询失败'
    await loadHistory()
  } finally {
    asking.value = false
  }
}

function onKeydown(event) {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault()
    submit()
  }
}

function useSuggestion(text) {
  question.value = text
}

function reuse(item) {
  question.value = item.question
  if (item.datasource_id) datasourceId.value = item.datasource_id
}

watch(datasourceId, (value) => {
  if (value) localStorage.setItem('chatbi-datasource', String(value))
  loadTables()
})

watch(model, (value) => localStorage.setItem('chatbi-model', value))

onMounted(boot)
</script>

<template>
  <div class="panel-page">
    <PageHead title="问数" />
    <div class="panel-body">
      <header class="panel-intro">
        <h1>问数</h1>
        <p>本库问数读取当前应用里的全部数据表并生成图表。外部数据库在下面单独查询，只执行单条只读 SQL。</p>
      </header>
      <section class="panel-card catalog-ask">
        <h2>本库</h2>
        <p>问表、字段、数量或图表。密码哈希和登录令牌只显示为已隐藏。</p>
        <form class="ask-box" @submit.prevent="submitCatalog">
          <textarea v-model="catalogQuestion" rows="3" maxlength="1000" placeholder="例如：读取系统全部数据表，并用图表展示" />
          <div class="ask-actions">
            <span>只读查询</span>
            <button class="primary" type="submit" :disabled="catalogAsking">{{ catalogAsking ? '查询中' : '问数' }}</button>
          </div>
        </form>
        <div class="chips">
          <button v-for="item in catalogIdeas" :key="item" type="button" @click="catalogQuestion = item">{{ item }}</button>
        </div>
        <p v-if="catalogError" class="auth-error">{{ catalogError }}</p>
        <template v-if="catalogResult">
          <p v-if="!catalogResult.matched" class="result-summary">{{ catalogResult.answer }}</p>
          <template v-else>
            <ChartPanel v-if="catalogResult.chart" :chart="catalogResult.chart" />
            <div v-if="catalogResult.answer" class="markdown" v-html="catalogHtml" />
          </template>
        </template>
      </section>
      <h2 class="section-title">外部数据源</h2>
      <div v-if="!chatbi.sources.length" class="panel-card empty-card">
        <strong>还没有数据源</strong>
        <p>先添加 MySQL、PostgreSQL 或 SQL Server，再回来提问。</p>
        <button class="primary" type="button" @click="router.push('/datasources')">配置数据源</button>
      </div>
      <template v-else>
        <div class="ask-toolbar">
          <label>
            数据源
            <select v-model="datasourceId">
              <option v-for="item in chatbi.sources" :key="item.id" :value="item.id">
                {{ item.name }} · {{ item.driver }}
              </option>
            </select>
          </label>
          <label>
            模型
            <select v-model="model">
              <option v-for="item in models" :key="item.id" :value="item.id">{{ item.label }}</option>
            </select>
          </label>
          <button class="balance-chip" type="button" @click="router.push('/billing')">
            余额 {{ chatbi.balance ?? '—' }}
            <small>每次成功 {{ chatbi.askCost }}</small>
          </button>
        </div>
        <form class="ask-box" @submit.prevent="submit">
          <textarea
            v-model="question"
            rows="3"
            maxlength="500"
            placeholder="例如：orders 每个 city 的销售额"
            @keydown="onKeydown"
          />
          <div class="ask-actions">
            <span v-if="source">{{ source.host }}:{{ source.port }} / {{ source.database }}</span>
            <button class="primary" type="submit" :disabled="asking">{{ asking ? '查询中' : '查询' }}</button>
          </div>
        </form>
        <div class="chips">
          <button v-for="item in suggestions" :key="item" type="button" @click="useSuggestion(item)">{{ item }}</button>
        </div>
        <p v-if="errorText" class="auth-error">{{ errorText }}</p>
        <section v-if="result" class="panel-card result-card">
          <p class="result-summary">{{ result.summary }}</p>
          <pre v-if="result.sql" class="sql-block">{{ result.sql }}</pre>
          <div v-if="result.chart === 'metric'" class="metric">
            <strong>{{ cellText(result.rows?.[0]?.[result.columns.length - 1]) }}</strong>
            <span>{{ result.columns?.[result.columns.length - 1] }}</span>
          </div>
          <div v-else-if="result.chart === 'line' && bars.length" class="line-chart">
            <svg viewBox="0 0 320 120" role="img">
              <polyline :points="linePoints" />
            </svg>
            <div class="line-labels">
              <span v-for="item in bars" :key="item.label">{{ item.label }}</span>
            </div>
          </div>
          <div v-else-if="result.chart === 'bar' && bars.length" class="bar-list">
            <div v-for="item in bars" :key="item.label" class="bar-row">
              <span>{{ item.label }}</span>
              <div><i :style="{ width: item.width + '%' }" /></div>
              <strong>{{ item.value }}</strong>
            </div>
          </div>
          <div v-if="result.columns?.length" class="table-wrap">
            <table class="data-table">
              <thead>
                <tr>
                  <th v-for="column in result.columns" :key="column">{{ column }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, index) in result.rows" :key="index">
                  <td v-for="(cell, cellIndex) in row" :key="cellIndex">{{ cellText(cell) }}</td>
                </tr>
                <tr v-if="!result.rows?.length">
                  <td :colspan="result.columns.length">没有数据</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
        <section class="history-block">
          <h2>最近提问</h2>
          <button v-for="item in history" :key="item.id" class="history-row" type="button" @click="reuse(item)">
            <strong>{{ item.question }}</strong>
            <span>{{ item.datasource_name }} · {{ item.status === 'ok' ? '成功' : '失败' }} · {{ formatDateTime(item.created_at) }}</span>
          </button>
          <p v-if="!history.length" class="muted">还没有问数记录。</p>
        </section>
      </template>
    </div>
  </div>
</template>
