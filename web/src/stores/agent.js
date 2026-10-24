import { reactive } from 'vue'
import { post } from '../api/client'
import { chat, notify, resetDraft, setModel, stop } from './chat'

export const agents = reactive({
  items: [],
})

export async function refreshAgents() {
  const result = await post('/api/agent/search', { page: 1, page_size: 100 })
  agents.items = result.data || []
  return agents.items
}

export async function loadAgent(id) {
  const result = await post('/api/agent/one', { id: String(id) })
  return result.data
}

export async function saveAgent(body) {
  const path = body.id ? '/api/agent/update' : '/api/agent/create'
  const payload = {
    name: body.name,
    description: body.description,
    instructions: body.instructions,
    model: body.model,
    opening: body.opening,
    suggestions: body.suggestions,
  }
  if (body.id) payload.id = String(body.id)
  const result = await post(path, payload)
  await refreshAgents()
  return result.data
}

export async function removeAgent(id) {
  await post('/api/agent/delete', { id: String(id) })
  agents.items = agents.items.filter((item) => String(item.id) !== String(id))
  if (String(chat.agentId) === String(id)) chat.agentId = ''
}

export function findAgent(id) {
  return agents.items.find((item) => String(item.id) === String(id)) || null
}

export function openAgentChat(router, agent) {
  stop()
  resetDraft()
  chat.agentId = String(agent.id)
  if (agent.model) setModel(agent.model)
  chat.sidebarOpen = false
  router.push({ path: '/', query: { agent: String(agent.id) } })
}

export function agentError(error, fallback) {
  notify(error?.message || fallback)
}
