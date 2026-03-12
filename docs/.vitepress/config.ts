import { defineConfig } from 'vitepress'

export default defineConfig({
  ignoreDeadLinks: true,
  title: 'Maggus',
  description: 'AI-powered task automation CLI that orchestrates Claude Code to work through implementation plans',

  themeConfig: {
    logo: '/avatar.png',

    nav: [
      { text: 'Home', link: '/' },
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'Reference', link: '/reference/commands' }
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Guide',
          items: [
            { text: 'Getting Started', link: '/guide/getting-started' },
            { text: 'Writing Plans', link: '/guide/writing-plans' },
            { text: 'Concepts', link: '/guide/concepts' }
          ]
        }
      ],
      '/reference/': [
        {
          text: 'Reference',
          items: [
            { text: 'CLI Commands', link: '/reference/commands' },
            { text: 'Configuration', link: '/reference/configuration' }
          ]
        }
      ]
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/dirnei/maggus' }
    ]
  }
})
