import './style.css'
import { mount } from 'svelte'
import App from './App.svelte'

const target = document.getElementById('app')

if (!target) {
  throw new Error('FlowPilot root element (#app) was not found')
}

document.documentElement.dataset.app = 'flowpilot'
document.body.dataset.theme = 'dark'

mount(App, {
  target,
})
