import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import './assets/styles.css'

const saved = localStorage.getItem('chatbi-theme')
document.documentElement.dataset.theme = saved === 'dark' ? 'dark' : 'light'

createApp(App).use(router).mount('#app')
