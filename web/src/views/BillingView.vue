<script setup>
import { onMounted, ref } from 'vue'
import PageHead from '../components/PageHead.vue'
import { notify } from '../stores/chat'
import { chatbi, kindLabel, refreshBalance, searchBilling, topup } from '../stores/chatbi'
import { formatDateTime } from '../utils/time'

const entries = ref([])
const total = ref(0)
const page = ref(1)
const amount = ref(100)
const note = ref('')
const saving = ref(false)
const presets = [100, 500, 1000, 5000]

async function load() {
  try {
    const [ledger] = await Promise.all([searchBilling(page.value), refreshBalance()])
    entries.value = ledger.data || []
    total.value = ledger.total || 0
  } catch (error) {
    notify(error.message || '加载账单失败')
  }
}

async function recharge() {
  saving.value = true
  try {
    const result = await topup(amount.value, note.value.trim())
    chatbi.balance = result.data.balance
    note.value = ''
    page.value = 1
    notify('已记入充值')
    await load()
  } catch (error) {
    notify(error.message || '充值失败')
  } finally {
    saving.value = false
  }
}

function prev() {
  if (page.value <= 1) return
  page.value -= 1
  load()
}

function next() {
  if (page.value * 20 >= total.value) return
  page.value += 1
  load()
}

onMounted(load)
</script>

<template>
  <div class="panel-page">
    <PageHead title="账单" />
    <div class="panel-body">
      <header class="panel-intro">
        <h1>账户账单</h1>
        <p>首次打开赠送 {{ chatbi.welcome }} 额度。问数成功扣 {{ chatbi.askCost }}。这里的充值会立刻写入账单并增加可用额度，适合自建环境记账。</p>
      </header>
      <section class="balance-hero">
        <span>当前余额</span>
        <strong>{{ chatbi.balance ?? '—' }}</strong>
      </section>
      <form class="panel-card topup-card" @submit.prevent="recharge">
        <h2>记一笔充值</h2>
        <div class="chips">
          <button v-for="item in presets" :key="item" type="button" :class="{ selected: Number(amount) === item }" @click="amount = item">{{ item }}</button>
        </div>
        <div class="field-row">
          <label class="field">额度<input v-model.number="amount" type="number" min="1" max="100000" required /></label>
          <label class="field">备注<input v-model="note" maxlength="80" placeholder="例如：十月额度" /></label>
        </div>
        <button class="primary" type="submit" :disabled="saving">{{ saving ? '提交中' : '确认充值' }}</button>
      </form>
      <section>
        <div class="section-head">
          <h2>流水</h2>
          <div class="pager">
            <button type="button" :disabled="page <= 1" @click="prev">上一页</button>
            <span>{{ page }}</span>
            <button type="button" :disabled="page * 20 >= total" @click="next">下一页</button>
          </div>
        </div>
        <div class="table-wrap">
          <table class="data-table">
            <thead>
              <tr>
                <th>时间</th>
                <th>类型</th>
                <th>额度</th>
                <th>余额</th>
                <th>备注</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in entries" :key="item.id">
                <td>{{ formatDateTime(item.created_at) }}</td>
                <td><span class="pill" :class="item.kind">{{ kindLabel[item.kind] || item.kind }}</span></td>
                <td>{{ item.kind === 'consume' ? '-' : '+' }}{{ item.amount }}</td>
                <td>{{ item.balance_after }}</td>
                <td>{{ item.note }}</td>
              </tr>
              <tr v-if="!entries.length">
                <td colspan="5">还没有账单。</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </div>
</template>
