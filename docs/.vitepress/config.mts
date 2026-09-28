import { defineConfig } from 'vitepress'

const repo = 'https://github.com/krank56/jenklod-batman'

export default defineConfig({
  title: 'jenklod-batman',
  description: 'A terminal UI for Jenkins: watched jobs, search across every folder, macros, and pipeline input approvals.',
  base: '/',
  sitemap: { hostname: 'https://jenklod-batman.ytalbi.com' },
  lang: 'en-US',
  cleanUrls: true,
  lastUpdated: true,
  appearance: 'dark',
  srcExclude: ['tapes/**', 'README.md'],
  head: [
    ['link', { rel: 'icon', type: 'image/png', href: '/logo-mark.png' }],
    ['meta', { name: 'theme-color', content: '#FFD500' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:title', content: 'jenklod-batman — Jenkins, from the Batcave' }],
    ['meta', { property: 'og:image', content: 'https://jenklod-batman.ytalbi.com/banner-dark.png' }],
  ],
  themeConfig: {
    logo: '/logo-mark.png',
    nav: [
      { text: 'Guide', link: '/guide/install', activeMatch: '/guide/' },
      { text: 'Reference', link: '/reference/keys', activeMatch: '/reference/' },
      { text: 'Contributing', link: '/contributing/development', activeMatch: '/contributing/' },
      { text: 'Releases', link: `${repo}/releases` },
    ],
    sidebar: [
      {
        text: 'Getting started',
        items: [
          { text: 'Install', link: '/guide/install' },
          { text: 'First run', link: '/guide/first-run' },
        ],
      },
      {
        text: 'Guide',
        items: [
          { text: 'Browsing & builds', link: '/guide/browsing' },
          { text: 'Watching jobs', link: '/guide/watching' },
          { text: 'Search', link: '/guide/search' },
          { text: 'Pipeline inputs', link: '/guide/inputs' },
          { text: 'Macros', link: '/guide/macros' },
        ],
      },
      {
        text: 'Reference',
        items: [
          { text: 'Keys', link: '/reference/keys' },
          { text: 'Command line', link: '/reference/cli' },
          { text: 'config.toml', link: '/reference/config' },
        ],
      },
      {
        text: 'Contributing',
        items: [
          { text: 'Development', link: '/contributing/development' },
          { text: 'Releasing', link: '/contributing/releasing' },
        ],
      },
    ],
    socialLinks: [{ icon: 'github', link: repo }],
    search: { provider: 'local' },
    editLink: {
      pattern: `${repo}/edit/main/docs/:path`,
      text: 'Edit this page on GitHub',
    },
    footer: {
      message: 'Released under the MIT License. Gotham, its jobs and its people in these pages are fictional.',
      copyright: 'jenklod-batman contributors',
    },
    outline: { level: [2, 3] },
  },
})
