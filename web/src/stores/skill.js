import { reactive } from 'vue'
import { post } from '../api/client'

export const skills = reactive({
  items: [],
})

export const skillSamples = {
  python: `import json
import sys

data = json.load(sys.stdin)
text = str(data.get("input", ""))
print(text.upper())
`,
  node: `let raw = ""
process.stdin.setEncoding("utf8")
process.stdin.on("data", (chunk) => {
  raw += chunk
})
process.stdin.on("end", () => {
  const data = JSON.parse(raw || "{}")
  process.stdout.write(String(data.input ?? "").toUpperCase())
})
`,
  bash: `input=$(cat)
printf "%s\\n" "$input"
`,
}

export const defaultParameters = `{
  "type": "object",
  "properties": {
    "input": {
      "type": "string",
      "description": "传给脚本的文本"
    }
  },
  "additionalProperties": true
}`

export async function refreshSkills() {
  const result = await post('/api/skill/search', { page: 1, page_size: 100 })
  skills.items = result.data || []
  return skills.items
}

export async function loadSkill(id) {
  const result = await post('/api/skill/one', { id: String(id) })
  return result.data
}

export async function saveSkill(body) {
  const path = body.id ? '/api/skill/update' : '/api/skill/create'
  const payload = {
    title: body.title,
    name: body.name,
    description: body.description,
    instructions: body.instructions,
    runtime: body.runtime,
    script: body.script,
    parameters: body.parameters,
    timeout_sec: Number(body.timeout_sec) || 10,
    enabled: !!body.enabled,
  }
  if (body.id) payload.id = String(body.id)
  const result = await post(path, payload)
  await refreshSkills()
  return result.data
}

export async function removeSkill(id) {
  await post('/api/skill/delete', { id: String(id) })
  skills.items = skills.items.filter((item) => String(item.id) !== String(id))
}

export async function runSkill(body) {
  const result = await post('/api/skill/run', {
    id: body.id ? String(body.id) : undefined,
    name: body.name,
    runtime: body.runtime,
    script: body.script,
    input: body.input,
    timeout_sec: Number(body.timeout_sec) || 10,
  })
  return result.data
}
