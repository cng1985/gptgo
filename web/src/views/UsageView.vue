<script setup>
import { computed, onMounted, ref } from 'vue'
import PageHead from '../components/PageHead.vue'
import { notify } from '../stores/chat'
import { chatbi, refreshBalance, searchUsage, usageSummary } from '../stores/chatbi'
import { formatDateTime } from '../utils/time'

const days = ref(14)
const summary = ref(null)
const records = ref([])
const dayChoices = [7, 14, 30]

const maxQuery = computed(() => Math.max(...(summary.value?.days || []).map((item) => item.query_count), 1))

async function load() {
  try {
    const [stats, page] = await Promise.all([
      usageSummary(days.value),
      searchUsage({ page: 1, pageSize: 20 }),
      refreshBalance(),
    ])
    summary.value = stats.data
    records.value = page.data || []
    chatbi.balance = stats.data.balance
  } catch (error) {
    notify(error.message || '加载用量失败')
  }
}

function chooseDays(value) {
  days.value = value
  load()
}

onMounted(load)
</script>

<template>
  <div class="panel-page">
    <PageHead title="用量统计" />
    <div class="panel-body">
      <header class="panel-intro">
        <h1>用量统计</h1>
        <p>按 UTC 日期汇总当前账号的问数次数、估算令牌和扣费。失败的查询会计入次数，但不会扣额度。</p>
      </header>
      <div class="chips">
        <button v-for="item in dayChoices" :key="item" type="button" :class="{ selected: days === item }" @click="chooseDays(item)">近 {{ item }} 天</button>
      </div>
      <div v-if="summary" class="stat-grid">
        <article><span>查询</span><strong>{{ summary.query_count }}</strong></article>
        <article><span>成功 / 失败</span><strong>{{ summary.success_count }} / {{ summary.fail_count }}</strong></article>
        <article><span>估算令牌</span><strong>{{ summary.token_count }}</strong></article>
        <article><span>扣费 / 余额</span><strong>{{ summary.cost }} / {{ summary.balance }}</strong></article>
      </div>
      <section v-if="summary" class="panel-card">
        <h2>每日查询</h2>
        <div class="day-chart">
          <div v-for="item in summary.days" :key="item.date" class="day-col" :title="`${item.date} ${item.query_count} 次`">
            <i :style="{ height: Math.max(4, (item.query_count / maxQuery) * 100) + '%' }" />
            <small>{{ item.date.slice(5) }}</small>
          </div>
        </div>
      </section>
      <section v-if="summary">
        <h2>按数据源</h2>
        <div class="table-wrap">
          <table class="data-table">
            <thead>
              <tr>
                <th>数据源</th>
                <th>查询</th>
                <th>成功</th>
                <th>扣费</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in summary.datasources" :key="item.datasource_id + item.datasource_name">
                <td>{{ item.datasource_name }}</td>
                <td>{{ item.query_count }}</td>
                <td>{{ item.success_count }}</td>
                <td>{{ item.cost }}</td>
              </tr>
              <tr v-if="!summary.datasources.length">
                <td colspan="4">这段时间还没有用量。</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
      <section>
        <h2>明细</h2>
        <div class="table-wrap">
          <table class="data-table">
            <thead>
              <tr>
                <th>时间</th>
                <th>数据源</th>
                <th>问题</th>
                <th>状态</th>
                <th>行数</th>
                <th>令牌</th>
                <th>扣费</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in records" :key="item.id">
                <td>{{ formatDateTime(item.created_at) }}</td>
                <td>{{ item.datasource_name }}</td>
                <td class="question-cell" :title="item.sql || item.error_message">{{ item.question }}</td>
                <td><span class="pill" :class="item.status">{{ item.status === 'ok' ? '成功' : '失败' }}</span></td>
                <td>{{ item.row_count }}</td>
                <td>{{ item.prompt_tokens + item.completion_tokens }}</td>
                <td>{{ item.cost }}</td>
              </tr>
              <tr v-if="!records.length">
                <td colspan="7">还没有问数记录。</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </div>
</template>
