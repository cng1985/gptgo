<script setup>
import { computed } from 'vue'

const props = defineProps({
  chart: { type: Object, required: true },
})

const palette = ['#6c64f2', '#0d7c66', '#e09f3e', '#3d7ea6', '#d14343', '#c06c84', '#7a8450', '#8b6cc1', '#d17a3a', '#4c8c8a', '#6a7fdb', '#b56b6b']

const tables = computed(() => props.chart.tables || [])
const columns = computed(() => props.chart.columns || [])
const rows = computed(() => props.chart.rows || [])

const series = computed(() => {
  const xi = columns.value.indexOf(props.chart.x)
  const yi = columns.value.indexOf(props.chart.y)
  if (xi < 0 || yi < 0) return []
  return rows.value.map((row) => ({
    label: row[xi] == null || row[xi] === '' ? '—' : String(row[xi]),
    value: numeric(row[yi]),
  })).filter((item) => item.value != null)
})

const kind = computed(() => {
  const type = props.chart.type || 'table'
  if ((type === 'bar' || type === 'line' || type === 'pie') && series.value.length === 0) return 'table'
  return type
})

const bars = computed(() => {
  const shown = series.value.slice(0, 12)
  const max = Math.max(...shown.map((item) => item.value), 1)
  return shown.map((item) => ({
    ...item,
    width: Math.max(2, Math.round((item.value / max) * 340)),
    text: formatValue(item.value),
    label: clip(item.label, 18),
  }))
})

const barHeight = computed(() => Math.max(48, bars.value.length * 32 + 8))

const line = computed(() => {
  const shown = series.value.slice(0, 48)
  const max = niceMax(Math.max(...shown.map((item) => item.value), 0))
  const width = 640
  const height = 240
  const pad = { l: 44, r: 16, t: 16, b: 32 }
  const innerW = width - pad.l - pad.r
  const innerH = height - pad.t - pad.b
  const points = shown.map((item, index) => {
    const x = pad.l + (shown.length === 1 ? innerW / 2 : (innerW * index) / (shown.length - 1))
    const y = pad.t + innerH - (item.value / max) * innerH
    return { ...item, x, y, text: formatValue(item.value) }
  })
  const ticks = [0, 0.5, 1].map((ratio) => ({
    y: pad.t + innerH - innerH * ratio,
    text: formatValue(max * ratio),
  }))
  const labels = []
  if (points.length) {
    const indexes = [0, Math.floor((points.length - 1) / 2), points.length - 1]
    const seen = new Set()
    for (const index of indexes) {
      if (seen.has(index)) continue
      seen.add(index)
      labels.push({ x: points[index].x, text: clip(points[index].label, 12) })
    }
  }
  return {
    width,
    height,
    pad,
    innerW,
    points,
    ticks,
    labels,
    d: points.map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x} ${point.y}`).join(' '),
  }
})

const slices = computed(() => {
  const shown = series.value.slice(0, 8)
  const extra = series.value.slice(8).reduce((sum, item) => sum + item.value, 0)
  const items = extra > 0 ? [...shown, { label: '其他', value: extra }] : shown
  const total = items.reduce((sum, item) => sum + item.value, 0) || 1
  let angle = -Math.PI / 2
  return items.map((item, index) => {
    const sweep = (item.value / total) * Math.PI * 2
    const start = angle
    const end = angle + sweep
    angle = end
    return {
      ...item,
      path: sweep >= Math.PI * 2 - 0.0001 ? '' : arcPath(150, 120, 86, start, end),
      full: sweep >= Math.PI * 2 - 0.0001,
      color: palette[index % palette.length],
      text: formatValue(item.value),
      label: clip(item.label, 16),
    }
  })
})

const metricText = computed(() => {
  const row = rows.value[0]
  if (!row) return '—'
  const index = columns.value.indexOf(props.chart.y)
  return formatValue(index >= 0 ? row[index] : row[0])
})

const metricLabel = computed(() => props.chart.y || columns.value[0] || '')

function numeric(value) {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && /^-?\d+(\.\d+)?$/.test(value.trim())) return Number(value)
  return null
}

function formatValue(value) {
  if (value == null || value === '') return '—'
  const number = typeof value === 'number' ? value : numeric(value)
  if (number == null) return String(value)
  if (Number.isInteger(number)) return number.toLocaleString('zh-CN')
  return number.toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}

function clip(text, size) {
  const chars = Array.from(text || '')
  if (chars.length <= size) return chars.join('')
  return `${chars.slice(0, size).join('')}…`
}

function niceMax(value) {
  if (value <= 0) return 1
  const exp = 10 ** Math.floor(Math.log10(value))
  const fraction = value / exp
  const nice = fraction <= 1 ? 1 : fraction <= 2 ? 2 : fraction <= 5 ? 5 : 10
  return nice * exp
}

function arcPath(cx, cy, radius, start, end) {
  const x1 = cx + radius * Math.cos(start)
  const y1 = cy + radius * Math.sin(start)
  const x2 = cx + radius * Math.cos(end)
  const y2 = cy + radius * Math.sin(end)
  const large = end - start > Math.PI ? 1 : 0
  return `M ${cx} ${cy} L ${x1} ${y1} A ${radius} ${radius} 0 ${large} 1 ${x2} ${y2} Z`
}

function cell(row, index) {
  return formatValue(row[index])
}
</script>

<template>
  <section class="ask-chart">
    <div class="ask-kicker">问数</div>
    <h3>{{ chart.title || '查询结果' }}</h3>
    <p v-if="kind === 'bar' && series.length > bars.length" class="ask-note">柱状图展示前 {{ bars.length }} 项，表格保留全部 {{ series.length }} 项。</p>
    <p v-else-if="kind === 'line' && series.length > line.points.length" class="ask-note">折线图展示前 {{ line.points.length }} 个点，表格保留全部数据。</p>
    <svg v-if="kind === 'bar'" class="ask-plot" :viewBox="`0 0 640 ${barHeight}`" role="img" :aria-label="chart.title || '柱状图'">
      <g v-for="(item, index) in bars" :key="`${item.label}-${index}`">
        <text :x="8" :y="index * 32 + 24">{{ item.label }}</text>
        <rect :x="148" :y="index * 32 + 10" :width="item.width" height="16" rx="4" :fill="palette[index % palette.length]" />
        <text :x="156 + item.width" :y="index * 32 + 23">{{ item.text }}</text>
      </g>
    </svg>
    <svg v-else-if="kind === 'line'" class="ask-plot" :viewBox="`0 0 ${line.width} ${line.height}`" role="img" :aria-label="chart.title || '折线图'">
      <line v-for="tick in line.ticks" :key="tick.y" :x1="line.pad.l" :x2="line.width - line.pad.r" :y1="tick.y" :y2="tick.y" class="ask-grid" />
      <text v-for="tick in line.ticks" :key="`t-${tick.y}`" :x="4" :y="tick.y + 4">{{ tick.text }}</text>
      <path :d="line.d" fill="none" stroke="#6c64f2" stroke-width="2.5" />
      <circle v-for="(point, index) in line.points" :key="index" :cx="point.x" :cy="point.y" r="3.5" fill="#6c64f2" />
      <text v-for="(label, index) in line.labels" :key="`l-${index}`" :x="label.x" :y="line.height - 8" text-anchor="middle">{{ label.text }}</text>
    </svg>
    <div v-else-if="kind === 'pie'" class="ask-pie">
      <svg viewBox="0 0 300 240" role="img" :aria-label="chart.title || '饼图'">
        <circle v-if="slices.length === 1 && slices[0].full" cx="150" cy="120" r="86" :fill="slices[0].color" />
        <path v-for="(slice, index) in slices.filter((item) => !item.full)" :key="index" :d="slice.path" :fill="slice.color" />
      </svg>
      <ul>
        <li v-for="(slice, index) in slices" :key="`${slice.label}-${index}`">
          <i :style="{ background: slice.color }" />
          <span>{{ slice.label }}</span>
          <strong>{{ slice.text }}</strong>
        </li>
      </ul>
    </div>
    <div v-else-if="kind === 'metric'" class="ask-metric">
      <strong>{{ metricText }}</strong>
      <span>{{ metricLabel }}</span>
    </div>
    <div v-if="columns.length && rows.length" class="ask-table-wrap">
      <table class="ask-table">
        <thead>
          <tr>
            <th v-for="column in columns" :key="column" scope="col">{{ column }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, index) in rows" :key="index">
            <td v-for="(column, columnIndex) in columns" :key="column">{{ cell(row, columnIndex) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <p v-else-if="kind !== 'metric'" class="ask-note">这次查询没有返回行。</p>
    <details v-if="tables.length" class="ask-schema">
      <summary>已读取全部 {{ tables.length }} 张数据表</summary>
      <ul>
        <li v-for="table in tables" :key="table.name">
          <strong>{{ table.name }}</strong>
          <em>{{ table.rows }} 条</em>
          <span>{{ (table.columns || []).join('、') }}</span>
        </li>
      </ul>
    </details>
    <details v-if="chart.sql" class="ask-sql">
      <summary>查询</summary>
      <pre>{{ chart.sql }}</pre>
    </details>
    <p v-if="chart.note" class="ask-note">{{ chart.note }}</p>
  </section>
</template>
