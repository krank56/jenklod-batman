import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import Demo from './components/Demo.vue'
import './style.css'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('Demo', Demo)
  },
} satisfies Theme
