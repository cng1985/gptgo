import { createRouter, createWebHistory } from 'vue-router'
import AppLayout from '../layouts/AppLayout.vue'
import ChatView from '../views/ChatView.vue'
import WorkspaceView from '../views/WorkspaceView.vue'
import AuthView from '../views/AuthView.vue'
import ChatBIView from '../views/ChatBIView.vue'
import DatasourceView from '../views/DatasourceView.vue'
import BillingView from '../views/BillingView.vue'
import UsageView from '../views/UsageView.vue'
import AgentView from '../views/AgentView.vue'
import { ensureSession } from '../stores/chat'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: AuthView, meta: { guest: true } },
    { path: '/register', name: 'register', component: AuthView, meta: { guest: true } },
    {
      path: '/',
      component: AppLayout,
      children: [
        { path: '', name: 'new-chat', component: ChatView },
        { path: 'c/:id', name: 'conversation', component: ChatView },
        { path: 'workspace', name: 'workspace', component: WorkspaceView },
        { path: 'agents', name: 'agents', component: AgentView },
        { path: 'bi', name: 'bi', component: ChatBIView },
        { path: 'datasources', name: 'datasources', component: DatasourceView },
        { path: 'billing', name: 'billing', component: BillingView },
        { path: 'usage', name: 'usage', component: UsageView },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  const authed = await ensureSession()
  if (to.meta.guest) {
    return authed ? { path: '/' } : true
  }
  if (!authed) {
    return { name: 'login', query: to.fullPath && to.fullPath !== '/' ? { redirect: to.fullPath } : {} }
  }
  return true
})
