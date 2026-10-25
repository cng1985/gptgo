import { reactive } from 'vue'
import { post, stream } from '../api/client'

export const models = [
  { id: 'minimax-m3.1-flash', label: 'MiniMax-M3.1-Flash' },
  { id: 'minimax-m2.5', label: 'MiniMax-M2.5' },
  { id: 'deepseek-chat', label: 'DeepSeek-V3' },
  { id: 'builtin', label: '内置助手' },
]

export const thinkingModes = [
  { id: 'off', label: '不思考 · mini', hint: '直接回答，不展示推理' },
  { id: 'light', label: '轻量思考', hint: '回答前折叠一段短推理' },
  { id: 'deep', label: '深度思考', hint: '按步骤展开推理过程' },
]

const allowedExt = new Set([
  'txt', 'md', 'json', 'yaml', 'yml', 'go', 'java', 'sql', 'js', 'ts', 'tsx', 'jsx', 'vue',
  'py', 'sh', 'xml', 'csv', 'html', 'css', 'c', 'cpp', 'h', 'rs', 'toml', 'ini', 'log',
  'php', 'rb', 'mod', 'sum', 'ps1', 'cmd',
])

export const chat = reactive({
  conversations: [],
  messages: [],
  account: { id: '', name: '', username: '', provider: 'builtin', provider_ready: false },
  authenticated: false,
  sessionReady: false,
  activeId: '',
  draft: '',
  attachments: [],
  model: localStorage.getItem('chatbi-model') || 'minimax-m3.1-flash',
  thinking: localStorage.getItem('chatbi-thinking') || 'off',
  webSearch: localStorage.getItem('chatbi-web-search') === '1',
  agentId: '',
  keyword: '',
  searching: false,
  streaming: false,
  optimizing: false,
  booted: false,
  toast: '',
  theme: document.documentElement.dataset.theme || 'light',
  sidebarOpen: false,
})

let controller = null
let toastTimer = 0

export function notify(message) {
  chat.toast = message
  window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => {
    chat.toast = ''
  }, 2200)
}

export function setModel(id) {
  chat.model = id
  localStorage.setItem('chatbi-model', id)
}

export function setThinking(id) {
  chat.thinking = id
  localStorage.setItem('chatbi-thinking', id)
}

export function setWebSearch(on) {
  chat.webSearch = !!on
  localStorage.setItem('chatbi-web-search', chat.webSearch ? '1' : '0')
}

export function toggleWebSearch() {
  setWebSearch(!chat.webSearch)
}

export function toggleTheme() {
  chat.theme = chat.theme === 'dark' ? 'light' : 'dark'
  document.documentElement.dataset.theme = chat.theme
  localStorage.setItem('chatbi-theme', chat.theme)
}

let pendingSession = null

export function ensureSession() {
  if (chat.sessionReady) return Promise.resolve(chat.authenticated)
  if (!pendingSession) {
    pendingSession = loadAccount()
      .then(() => {
        chat.authenticated = true
        chat.sessionReady = true
        return true
      })
      .catch((error) => {
        pendingSession = null
        if (error.code === 401) {
          chat.authenticated = false
          chat.sessionReady = true
          pendingSession = Promise.resolve(false)
          return false
        }
        throw error
      })
  }
  return pendingSession
}

function resetWorkspace() {
  stop()
  chat.conversations = []
  chat.messages = []
  chat.activeId = ''
  chat.draft = ''
  chat.attachments = []
  chat.keyword = ''
  chat.agentId = ''
  chat.booted = false
  chat.sidebarOpen = false
}

export async function login(body) {
  const result = await post('/api/account/login', body)
  resetWorkspace()
  chat.account = result.data
  chat.authenticated = true
  chat.sessionReady = true
  return result.data
}

export async function register(body) {
  const result = await post('/api/account/register', body)
  resetWorkspace()
  chat.account = result.data
  chat.authenticated = true
  chat.sessionReady = true
  return result.data
}

export async function logout() {
  await post('/api/account/logout', {})
  resetWorkspace()
  chat.account = { id: '', name: '', username: '', provider: 'builtin', provider_ready: false }
  chat.authenticated = false
  chat.sessionReady = true
}

export async function boot() {
  if (chat.booted) return
  await Promise.all([loadAccount(), refreshConversations()])
  chat.authenticated = true
  chat.sessionReady = true
  chat.booted = true
}

export async function loadAccount() {
  const result = await post('/api/account/one', {})
  chat.account = result.data
}

export async function refreshConversations() {
  const result = await post('/api/conversation/search', {
    keyword: chat.keyword,
    page: 1,
    page_size: 100,
  })
  chat.conversations = result.data || []
}

export async function openConversation(id, { force = false } = {}) {
  if (!id) {
    chat.activeId = ''
    chat.messages = []
    return
  }
  if (!force && String(chat.activeId) === String(id) && chat.messages.length && chat.streaming) return
  const [one, page] = await Promise.all([
    post('/api/conversation/one', { id: String(id) }),
    post('/api/message/search', { conversation_id: String(id), page: 1, page_size: 100 }),
  ])
  let payload = page
  const last = Math.max(1, Math.ceil((page.total || 0) / (page.page_size || 100)))
  if (last > 1) {
    payload = await post('/api/message/search', {
      conversation_id: String(id),
      page: last,
      page_size: 100,
    })
  }
  chat.activeId = String(id)
  chat.messages = payload.data || []
  if (one.data?.model) setModel(one.data.model)
  if (one.data?.thinking_mode) setThinking(one.data.thinking_mode)
  if (one.data && Object.prototype.hasOwnProperty.call(one.data, 'web_search')) setWebSearch(one.data.web_search)
  const agentId = one.data?.agent_id
  chat.agentId = agentId && String(agentId) !== '0' ? String(agentId) : ''
}

export function resetDraft() {
  stop()
  chat.activeId = ''
  chat.messages = []
  chat.draft = ''
  chat.attachments = []
  chat.agentId = ''
  chat.sidebarOpen = false
}

export function stop() {
  if (controller) {
    controller.abort()
    controller = null
  }
}

export async function send({ content, regenerateId, editId } = {}) {
  const text = (content ?? chat.draft).trim()
  const files = chat.attachments.map((file) => ({
    name: file.name,
    media_type: 'text/plain',
    content: file.content,
  }))
  if (!regenerateId && !text && files.length === 0) return
  if (chat.streaming) return

  const optimisticUser = {
    id: `tmp-user-${Date.now()}`,
    role: 'user',
    content: text,
    thinking: '',
    status: 'done',
    attachments: files.map((file, index) => ({ id: `tmp-file-${index}`, name: file.name })),
  }
  const optimisticAssistant = {
    id: `tmp-assistant-${Date.now()}`,
    role: 'assistant',
    content: '',
    thinking: '',
    thinking_mode: chat.thinking,
    tools: [],
    sources: [],
    searchStatus: '',
    searchQuery: '',
    status: 'streaming',
    attachments: [],
  }
  let assistantIndex = -1
  if (regenerateId) {
    assistantIndex = chat.messages.findIndex((item) => String(item.id) === String(regenerateId))
    if (assistantIndex >= 0) {
      chat.messages[assistantIndex].content = ''
      chat.messages[assistantIndex].thinking = ''
      chat.messages[assistantIndex].thinking_mode = chat.thinking
      chat.messages[assistantIndex].chart = null
      chat.messages[assistantIndex].tools = []
      chat.messages[assistantIndex].sources = []
      chat.messages[assistantIndex].searchStatus = ''
      chat.messages[assistantIndex].searchQuery = ''
      chat.messages[assistantIndex].status = 'streaming'
    }
  } else if (editId) {
    const index = chat.messages.findIndex((item) => String(item.id) === String(editId))
    if (index >= 0) {
      chat.messages[index].content = text
      chat.messages[index].attachments = optimisticUser.attachments
      chat.messages.splice(index + 1)
    }
    chat.messages.push(optimisticAssistant)
    assistantIndex = chat.messages.length - 1
  } else {
    chat.messages.push(optimisticUser, optimisticAssistant)
    assistantIndex = chat.messages.length - 1
    chat.draft = ''
    chat.attachments = []
  }

  const body = {
    content: regenerateId ? '' : text,
    model: chat.model,
    thinking_mode: chat.thinking,
    web_search: chat.webSearch,
    attachments: regenerateId ? [] : files,
  }
  if (chat.activeId) body.conversation_id = String(chat.activeId)
  if (!chat.activeId && chat.agentId) body.agent_id = String(chat.agentId)
  if (regenerateId) body.regenerate_message_id = String(regenerateId)
  if (editId) body.edit_message_id = String(editId)

  controller = new AbortController()
  chat.streaming = true
  const assistant = () => (assistantIndex >= 0 ? chat.messages[assistantIndex] : null)
  try {
    await stream('/api/chat/stream', body, {
      signal: controller.signal,
      onEvent(event, payload) {
        const current = assistant()
        if (event === 'meta') {
          chat.activeId = payload.conversation_id
          if (!regenerateId && assistantIndex > 0) {
            chat.messages[assistantIndex - 1].id = payload.user_message_id
          }
          if (current) {
            current.id = payload.assistant_message_id
            if (payload.thinking_mode) current.thinking_mode = payload.thinking_mode
          }
          const existing = chat.conversations.find((item) => String(item.id) === String(payload.conversation_id))
          if (!existing) {
            chat.conversations.unshift({
              id: payload.conversation_id,
              title: payload.title || text || '新对话',
              model: chat.model,
              updated_at: new Date().toISOString(),
            })
          } else if (payload.title) {
            existing.title = payload.title
            existing.updated_at = new Date().toISOString()
          }
        }
        if (event === 'thinking' && current) current.thinking += payload.delta || ''
        if (event === 'delta' && current) current.content += payload.delta || ''
        if (event === 'chart' && current) current.chart = payload
        if (event === 'search' && current) {
          current.searchStatus = payload.status || ''
          if (payload.query) current.searchQuery = payload.query
          if (Array.isArray(payload.sources)) current.sources = payload.sources
        }
        if (event === 'tool' && current) {
          if (!Array.isArray(current.tools)) current.tools = []
          const index = current.tools.findIndex((item) => item.id === payload.id)
          if (index >= 0) current.tools.splice(index, 1, payload)
          else current.tools.push(payload)
        }
        if (event === 'error') {
          if (current) current.status = 'error'
          notify(payload.msg || '生成失败')
        }
        if (event === 'done' && current) {
          current.status = payload.status || 'done'
          current.id = payload.message_id || current.id
        }
      },
    })
    const current = assistant()
    if (current && current.status === 'streaming') current.status = 'done'
    if (current && !current.content && !current.thinking && chat.activeId) {
      chat.streaming = false
      await openConversation(chat.activeId, { force: true })
    }
    await refreshConversations()
  } catch (error) {
    const current = assistant()
    if (error.name === 'AbortError') {
      if (current && current.status === 'streaming') current.status = 'stopped'
    } else {
      if (current) {
        current.status = 'error'
        if (!current.content) current.content = '这次没有生成完整回答。请再试一次。'
      }
      notify(error.message || '发送失败')
    }
  } finally {
    chat.streaming = false
    controller = null
  }
}

export async function optimizeDraft() {
  const text = chat.draft.trim()
  if (!text || chat.optimizing) return
  chat.optimizing = true
  try {
    const result = await post('/api/chat/optimize', { content: text, model: chat.model })
    chat.draft = result.data.content
    notify('已优化提示词')
  } catch (error) {
    notify(error.message || '优化失败')
  } finally {
    chat.optimizing = false
  }
}

export async function renameConversation(id, title) {
  const result = await post('/api/conversation/update', { id: String(id), title })
  const found = chat.conversations.find((item) => String(item.id) === String(id))
  if (found) found.title = result.data.title
}

export async function removeConversation(id) {
  await post('/api/conversation/delete', { id: String(id) })
  chat.conversations = chat.conversations.filter((item) => String(item.id) !== String(id))
  if (String(chat.activeId) === String(id)) {
    chat.activeId = ''
    chat.messages = []
  }
}

export async function clearConversations() {
  await post('/api/conversation/clear', { confirm: true })
  chat.conversations = []
  chat.activeId = ''
  chat.messages = []
  notify('已清空对话')
}

export async function saveName(name) {
  const result = await post('/api/account/update', { name })
  chat.account = result.data
  notify('已更新名称')
}

export async function addFiles(fileList) {
  const incoming = Array.from(fileList || [])
  for (const file of incoming) {
    const ext = (file.name.split('.').pop() || '').toLowerCase()
    if (!allowedExt.has(ext)) {
      notify('仅支持文本与代码文件')
      continue
    }
    if (file.size > 100 * 1024) {
      notify('单个文件不能超过 100KB')
      continue
    }
    if (chat.attachments.length >= 5) {
      notify('最多附加 5 个文件')
      break
    }
    const content = await file.text()
    chat.attachments.push({ name: file.name, content })
  }
}

export function removeAttachment(index) {
  chat.attachments.splice(index, 1)
}

export function activeTitle() {
  const found = chat.conversations.find((item) => String(item.id) === String(chat.activeId))
  return found?.title || '新对话'
}
