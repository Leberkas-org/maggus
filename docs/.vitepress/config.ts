import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Maggus',
  description: 'AI-powered task automation CLI that orchestrates Claude Code to work through implementation plans',

  themeConfig: {
    logo: '/avatar.png',

    nav: [
      { text: 'Home', link: '/' },
      { text: 'Guide', link: '/guide/getting-started' }
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Guide',
          items: [
            { text: 'Getting Started', link: '/guide/getting-started' },
            { text: 'Writing Plans', link: '/guide/writing-plans' }
          ]
        }
      ]
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/dirnei/maggus' }
    ]
  }
})
