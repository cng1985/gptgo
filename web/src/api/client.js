export async function upload(url, file) {
  const body = new FormData()
  body.append('file', file)
  const response = await fetch(url, {
    method: 'POST',
    headers: { Accept: 'application/json' },
    credentials: 'same-origin',
    body,
  })
  return readJSON(response)
}

export async function post(url, body) {
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    credentials: 'same-origin',
    body: JSON.stringify(body ?? {}),
  })
  return readJSON(response)
}

async function readJSON(response) {
  let data = {}
  try {
    data = await response.json()
  } catch {
    data = {}
  }
  if (!response.ok || (typeof data.code === 'number' && data.code !== 200)) {
    const error = new Error(data.msg || '请求失败')
    error.code = typeof data.code === 'number' ? data.code : response.status
    throw error
  }
  return data
}

export async function stream(url, body, { signal, onEvent }) {
  const response = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
    credentials: 'same-origin',
    body: JSON.stringify(body),
    signal,
  })
  const type = response.headers.get('content-type') || ''
  if (!type.includes('text/event-stream')) {
    let data = {}
    try {
      data = await response.json()
    } catch {
      data = {}
    }
    throw new Error(data.msg || '发送失败')
  }
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  while (true) {
    const { value, done } = await reader.read()
    buffer += decoder.decode(value || new Uint8Array(), { stream: !done })
    buffer = buffer.replace(/\r\n/g, '\n').replace(/\r/g, '\n')
    const parts = buffer.split('\n\n')
    buffer = done ? '' : parts.pop() || ''
    for (const part of parts) emitSSE(part, onEvent)
    if (done) break
  }
}

function emitSSE(chunk, onEvent) {
  const text = chunk.trim()
  if (!text || text.startsWith(':')) return
  let event = 'message'
  const dataLines = []
  for (const line of text.split('\n')) {
    if (line.startsWith('event:')) event = line.slice(6).trim()
    if (line.startsWith('data:')) dataLines.push(line.slice(5).trim())
  }
  if (!dataLines.length) return
  let payload = {}
  try {
    payload = JSON.parse(dataLines.join('\n'))
  } catch {
    payload = { raw: dataLines.join('\n') }
  }
  onEvent(event, payload)
}
