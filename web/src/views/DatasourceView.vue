<script setup>
import { onMounted, reactive, ref } from 'vue'
import PageHead from '../components/PageHead.vue'
import { notify } from '../stores/chat'
import { chatbi, deleteDatasource, driverLabel, drivers, refreshSources, saveDatasource, testDatasource } from '../stores/chatbi'

const saving = ref(false)
const testing = ref(false)
const message = ref('')
const form = reactive(blankForm())

function blankForm() {
  return {
    id: '',
    name: '',
    driver: 'mysql',
    host: '127.0.0.1',
    port: 3306,
    database: '',
    username: '',
    password: '',
    ssl_mode: '',
  }
}

function resetForm() {
  Object.assign(form, blankForm())
  message.value = ''
}

function edit(item) {
  Object.assign(form, {
    id: item.id,
    name: item.name,
    driver: item.driver,
    host: item.host,
    port: item.port,
    database: item.database,
    username: item.username,
    password: '',
    ssl_mode: item.ssl_mode || (item.driver === 'mysql' ? '' : 'disable'),
  })
  message.value = ''
}

function onDriver(event) {
  const id = event.target.value
  const next = drivers.find((item) => item.id === id)
  const previous = drivers.find((item) => item.port === Number(form.port))
  form.driver = id
  if (!form.port || previous) form.port = next.port
  if (id === 'mysql') form.ssl_mode = form.ssl_mode === 'require' ? 'require' : ''
  else if (!form.ssl_mode || form.ssl_mode === '') form.ssl_mode = 'disable'
}

async function reload() {
  try {
    await refreshSources()
  } catch (error) {
    notify(error.message || '加载失败')
  }
}

async function save() {
  saving.value = true
  message.value = ''
  try {
    await saveDatasource({ ...form })
    notify(form.id ? '已更新数据源' : '已保存数据源')
    resetForm()
    await reload()
  } catch (error) {
    message.value = error.message || '保存失败'
  } finally {
    saving.value = false
  }
}

async function test() {
  testing.value = true
  message.value = ''
  try {
    const result = await testDatasource({ ...form })
    message.value = `${result.data.message}，读取到 ${result.data.table_count} 张表`
  } catch (error) {
    message.value = error.message || '连接失败'
  } finally {
    testing.value = false
  }
}

async function remove(item) {
  if (!window.confirm(`删除数据源「${item.name}」？已产生的用量和账单仍会保留。`)) return
  try {
    await deleteDatasource(item.id)
    if (String(form.id) === String(item.id)) resetForm()
    notify('已删除')
    await reload()
  } catch (error) {
    notify(error.message || '删除失败')
  }
}

onMounted(reload)
</script>

<template>
  <div class="panel-page">
    <PageHead title="数据源" />
    <div class="panel-body">
      <header class="panel-intro">
        <h1>我的数据源</h1>
        <p>MySQL、PostgreSQL 和 SQL Server 按当前登录账号分开保存。密码加密后入库，列表和接口都不会回传明文。</p>
      </header>
      <div class="split">
        <section>
          <article v-for="item in chatbi.sources" :key="item.id" class="panel-card source-card" :class="{ selected: String(form.id) === String(item.id) }">
            <div>
              <strong>{{ item.name }}</strong>
              <span>{{ driverLabel(item.driver) }} · {{ item.username }}@{{ item.host }}:{{ item.port }}/{{ item.database }}</span>
            </div>
            <div class="row-actions">
              <button type="button" @click="edit(item)">编辑</button>
              <button type="button" class="danger-text" @click="remove(item)">删除</button>
            </div>
          </article>
          <p v-if="!chatbi.sources.length" class="muted">还没有数据源。在右侧保存第一个连接。</p>
        </section>
        <form class="panel-card source-form" @submit.prevent="save">
          <h2>{{ form.id ? '编辑数据源' : '新建数据源' }}</h2>
          <label class="field">名称<input v-model="form.name" maxlength="40" required placeholder="销售库" /></label>
          <label class="field">类型
            <select :value="form.driver" @change="onDriver">
              <option v-for="item in drivers" :key="item.id" :value="item.id">{{ item.label }}</option>
            </select>
          </label>
          <div class="field-row">
            <label class="field">地址<input v-model="form.host" required placeholder="127.0.0.1" /></label>
            <label class="field port">端口<input v-model.number="form.port" type="number" min="1" max="65535" required /></label>
          </div>
          <label class="field">库名<input v-model="form.database" required placeholder="app" /></label>
          <label class="field">用户名<input v-model="form.username" required /></label>
          <label class="field">密码
            <input v-model="form.password" type="password" autocomplete="new-password" :placeholder="form.id ? '留空则不修改' : '数据库密码'" />
          </label>
          <label v-if="form.driver !== 'mysql'" class="field">SSL
            <select v-model="form.ssl_mode">
              <option value="disable">不加密</option>
              <option value="require">加密</option>
              <option v-if="form.driver === 'postgres'" value="verify-ca">校验 CA</option>
              <option v-if="form.driver === 'postgres'" value="verify-full">校验主机名</option>
            </select>
          </label>
          <label v-else class="field">SSL
            <select v-model="form.ssl_mode">
              <option value="">默认</option>
              <option value="require">加密</option>
            </select>
          </label>
          <p v-if="message" class="form-note">{{ message }}</p>
          <div class="form-actions">
            <button class="ghost" type="button" :disabled="testing" @click="test">{{ testing ? '测试中' : '测试连接' }}</button>
            <button class="ghost" type="button" @click="resetForm">清空</button>
            <button class="primary" type="submit" :disabled="saving">{{ saving ? '保存中' : '保存' }}</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
