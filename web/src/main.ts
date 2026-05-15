import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import './stores/theme' // apply theme before mount to avoid flash
import './style.css'

createApp(App).use(router).mount('#app')
