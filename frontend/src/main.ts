import { createApp } from 'vue'
import './companion.css'

// Auxiliary windows do not initialise home queries, player tabs or browsers.
const auxiliary = ['companion', 'overlay'].includes(new URLSearchParams(location.search).get('view') || '')
const screen = auxiliary ? import('./AuxiliaryApp.vue') : import('./App.vue')
screen.then(({ default: component }) => createApp(component).mount('#app'))
