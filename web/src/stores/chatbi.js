import { reactive } from 'vue'
import { post } from '../api/client'

export const drivers = [
  { id: 'mysql', label: 'MySQL', port: 3306 },
  { id: 'postgres', label: 'PostgreSQL', port: 5432 },
  { id: 'sqlserver', label: 'SQL Server', port: 1433 },
]

export const kindLabel = {
  grant: '赠送',
  topup: '充值',
  consume: '消费',
  refund: '退回',
}

export const chatbi = reactive({
  sources: [],
  balance: null,
  askCost: 2,
  welcome: 1000,
})

export function driverLabel(id) {
  return drivers.find((item) => item.id === id)?.label || id
}

export async function refreshSources(keyword = '') {
  const result = await post('/api/datasource/search', { keyword, page: 1, page_size: 50 })
  chatbi.sources = result.data || []
  return chatbi.sources
}

export async function refreshBalance() {
  const result = await post('/api/billing/one', {})
  chatbi.balance = result.data.balance
  chatbi.askCost = result.data.ask_cost
  chatbi.welcome = result.data.welcome_credits
  return result.data
}

export function saveDatasource(body) {
  const payload = { ...body, port: Number(body.port) || 0 }
  if (payload.id) payload.id = String(payload.id)
  else delete payload.id
  if (!payload.password) delete payload.password
  return post(payload.id ? '/api/datasource/update' : '/api/datasource/create', payload)
}

export function deleteDatasource(id) {
  return post('/api/datasource/delete', { id: String(id) })
}

export function testDatasource(body) {
  const payload = { ...body, port: Number(body.port) || 0 }
  if (payload.id) payload.id = String(payload.id)
  else delete payload.id
  if (!payload.password) delete payload.password
  return post('/api/datasource/test', payload)
}

export function loadSchema(id) {
  return post('/api/datasource/schema', { id: String(id) })
}

export function askQuestion(body) {
  return post('/api/chatbi/ask', {
    datasource_id: String(body.datasourceId),
    question: body.question,
    model: body.model,
  })
}

export function askCatalog(question, thinkingMode) {
  return post('/api/ask', { question, thinking_mode: thinkingMode || 'off' })
}

export function searchBilling(page = 1) {
  return post('/api/billing/search', { page, page_size: 20 })
}

export function topup(amount, note) {
  return post('/api/billing/topup', { amount: Number(amount), note })
}

export function usageSummary(days) {
  return post('/api/usage/summary', { days })
}

export function searchUsage(body = {}) {
  const payload = { keyword: body.keyword || '', page: body.page || 1, page_size: body.pageSize || 20 }
  if (body.datasourceId) payload.datasource_id = String(body.datasourceId)
  return post('/api/usage/search', payload)
}
