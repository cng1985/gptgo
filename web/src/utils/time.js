export function groupLabel(value) {
  const time = new Date(value)
  if (Number.isNaN(time.getTime())) return '更早'
  const now = new Date()
  const startToday = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const startYesterday = new Date(startToday.getTime() - 86400000)
  const startWeek = new Date(startToday.getTime() - 6 * 86400000)
  if (time >= startToday) return '今天'
  if (time >= startYesterday) return '昨天'
  if (time >= startWeek) return '最近 7 天'
  return '更早'
}

export function formatDateTime(value) {
  const time = new Date(value)
  if (Number.isNaN(time.getTime())) return ''
  return time.toLocaleString('zh-CN', { hour12: false })
}

export function formatClock(value) {
  const time = new Date(value)
  if (Number.isNaN(time.getTime())) return ''
  const hour = `${time.getHours()}`.padStart(2, '0')
  const minute = `${time.getMinutes()}`.padStart(2, '0')
  return `${hour}:${minute}`
}
