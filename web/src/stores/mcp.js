import { reactive } from 'vue'
import { post } from '../api/client'

export const mcp = reactive({
  items: [],
})

export async function refreshServers() {
  const result = await post('/api/mcp/search', { page: 1, page_size: 100 })
  mcp.items = result.data || []
  return mcp.items
}

export async function loadServer(id) {
  const result = await post('/api/mcp/one', { id: String(id) })
  return result.data
}

export async function saveServer(body) {
  const path = body.id ? '/api/mcp/update' : '/api/mcp/create'
  const payload = {
    name: body.name,
    transport: body.transport,
    url: body.url,
    command: body.command,
    args: body.args,
    enabled: body.enabled,
  }
  if (body.id) payload.id = String(body.id)
  if (body.headers) payload.headers = body.headers
  const result = await post(path, payload)
  await refreshServers()
  return result.data
}

export async function removeServer(id) {
  await post('/api/mcp/delete', { id: String(id) })
  mcp.items = mcp.items.filter((item) => String(item.id) !== String(id))
}

export async function testServer(body) {
  const payload = { ...body }
  if (payload.id) payload.id = String(payload.id)
  else delete payload.id
  const result = await post('/api/mcp/test', payload)
  return result.data
}
