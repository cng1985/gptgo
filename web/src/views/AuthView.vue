<script setup>
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SparkleMark from '../components/SparkleMark.vue'
import { login, register } from '../stores/chat'

const route = useRoute()
const router = useRouter()
const mode = computed(() => (route.name === 'register' ? 'register' : 'login'))
const username = ref('')
const name = ref('')
const password = ref('')
const confirm = ref('')
const error = ref('')
const pending = ref(false)

function nextPath() {
  const redirect = route.query.redirect
  if (typeof redirect !== 'string' || !redirect.startsWith('/') || redirect.startsWith('//')) return '/'
  if (redirect.startsWith('/login') || redirect.startsWith('/register')) return '/'
  return redirect
}

async function submit() {
  error.value = ''
  const user = username.value.trim()
  if (!/^[\u4e00-\u9fffA-Za-z0-9_]{2,20}$/.test(user)) {
    error.value = '用户名需要是 2 到 20 位字母、数字、下划线或汉字'
    return
  }
  if (password.value.length < 6 || password.value.length > 64) {
    error.value = '密码需要是 6 到 64 个字符'
    return
  }
  if (mode.value === 'register') {
    const display = name.value.trim()
    if (display && (display.length > 20 || /[\u0000-\u001f]/.test(display))) {
      error.value = '名称需要是 1 到 20 个字符'
      return
    }
    if (password.value !== confirm.value) {
      error.value = '两次输入的密码不一致'
      return
    }
  }
  pending.value = true
  try {
    if (mode.value === 'register') {
      await register({ username: user, password: password.value, name: name.value.trim() })
    } else {
      await login({ username: user, password: password.value })
    }
    router.push(nextPath())
  } catch (err) {
    error.value = err.message || '请求失败'
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <div class="auth-screen">
    <form class="auth-card" @submit.prevent="submit">
      <div class="auth-brand">
        <span class="brand-mark"><SparkleMark :size="16" /></span>
        <span>ChatBI AI</span>
      </div>
      <h1>{{ mode === 'register' ? '创建账号' : '登录' }}</h1>
      <p class="lead">{{ mode === 'register' ? '注册后只看得到你自己的对话。' : '使用账号进入你的对话。' }}</p>
      <label class="auth-field">
        用户名
        <input v-model="username" name="username" autocomplete="username" maxlength="20" placeholder="字母、数字或汉字" />
      </label>
      <label v-if="mode === 'register'" class="auth-field">
        显示名称
        <input v-model="name" name="name" autocomplete="nickname" maxlength="20" placeholder="选填，默认使用用户名" />
      </label>
      <label class="auth-field">
        密码
        <input
          v-model="password"
          name="password"
          type="password"
          :autocomplete="mode === 'register' ? 'new-password' : 'current-password'"
          maxlength="64"
          placeholder="至少 6 位"
        />
      </label>
      <label v-if="mode === 'register'" class="auth-field">
        确认密码
        <input v-model="confirm" name="confirm" type="password" autocomplete="new-password" maxlength="64" placeholder="再输入一次" />
      </label>
      <p v-if="error" class="auth-error">{{ error }}</p>
      <button class="primary auth-submit" type="submit" :disabled="pending">
        {{ pending ? '请稍候…' : mode === 'register' ? '注册并进入' : '登录' }}
      </button>
      <p class="auth-switch">
        <template v-if="mode === 'register'">
          已有账号？ <router-link to="/login">去登录</router-link>
        </template>
        <template v-else>
          还没有账号？ <router-link to="/register">注册</router-link>
        </template>
      </p>
    </form>
  </div>
</template>
