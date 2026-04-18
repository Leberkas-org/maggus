import { defineConfig } from 'vitepress'

export default defineConfig({
  base: '/',
  ignoreDeadLinks: true,
  title: 'Maggus',
  description: 'AI-powered task automation CLI that orchestrates AI coding agents to work through implementation plans',

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
            { text: 'Maggus Skills', link: '/guide/maggus-plan-skill' },
            { text: 'Concepts', link: '/guide/concepts' }
          ]
        }
      ],
      '/reference/': [
        {
          text: 'Reference',
          items: [
            { text: 'CLI Commands', link: '/reference/commands' },
            { text: 'Terminal UI', link: '/reference/tui' },
            { text: 'Configuration', link: '/reference/configuration' }
          ]
        }
      ]
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/leberkas-org/maggus' }
    ]
  }
})
