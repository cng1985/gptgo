import { reactive } from 'vue'
import { post } from '../api/client'

export const toolkit = reactive({
  builtins: [],
  https: [],
  servers: [],
  remote: [],
})

export async function refreshToolkit() {
  const [builtins, https, servers] = await Promise.all([
    post('/api/tool/search', { page: 1, page_size: 20 }),
    post('/api/httptool/search', { page: 1, page_size: 50 }),
    post('/api/mcp/search', { page: 1, page_size: 50 }),
  ])
  toolkit.builtins = builtins.data || []
  toolkit.https = https.data || []
  toolkit.servers = servers.data || []
}

export async function setBuiltin(name, enabled) {
  await post('/api/tool/update', { name, enabled })
  await refreshToolkit()
}

export async function loadHTTP(id) {
  const result = await post('/api/httptool/one', { id: String(id) })
  return result.data
}

export async function saveHTTP(body) {
  const path = body.id ? '/api/httptool/update' : '/api/httptool/create'
  const payload = {
    name: body.name,
    description: body.description,
    parameters: body.parameters,
    method: body.method,
    url: body.url,
    headers: body.headers,
    enabled: body.enabled,
  }
  if (body.id) payload.id = String(body.id)
  const result = await post(path, payload)
  await refreshToolkit()
  return result.data
}

export async function removeHTTP(id) {
  await post('/api/httptool/delete', { id: String(id) })
  toolkit.https = toolkit.https.filter((item) => String(item.id) !== String(id))
}

export async function loadMCP(id) {
  const result = await post('/api/mcp/one', { id: String(id) })
  return result.data
}

export async function saveMCP(body) {
  const path = body.id ? '/api/mcp/update' : '/api/mcp/create'
  const payload = {
    name: body.name,
    transport: body.transport,
    command: body.command,
    args: body.args,
    env: body.env,
    url: body.url,
    headers: body.headers,
    enabled: body.enabled,
  }
  if (body.id) payload.id = String(body.id)
  const result = await post(path, payload)
  await refreshToolkit()
  return result.data
}

export async function removeMCP(id) {
  await post('/api/mcp/delete', { id: String(id) })
  toolkit.servers = toolkit.servers.filter((item) => String(item.id) !== String(id))
  toolkit.remote = []
}

export async function testMCP(id) {
  const result = await post('/api/mcp/tools', { id: String(id) })
  toolkit.remote = result.data || []
  return toolkit.remote
}
