import { createRouter, createWebHistory } from 'vue-router'
import AppLayout from '../layouts/AppLayout.vue'
import ChatView from '../views/ChatView.vue'
import WorkspaceView from '../views/WorkspaceView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: AppLayout,
      children: [
        { path: '', name: 'new-chat', component: ChatView },
        { path: 'c/:id', name: 'conversation', component: ChatView },
        { path: 'workspace', name: 'workspace', component: WorkspaceView },
      ],
    },
  ],
})
