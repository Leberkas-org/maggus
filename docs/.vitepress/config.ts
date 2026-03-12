import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Maggus',
  description: 'AI-powered task automation CLI that orchestrates Claude Code to work through implementation plans',

  themeConfig: {
    logo: '/avatar.png',

    nav: [
      { text: 'Home', link: '/' }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/dirnei/maggus' }
    ]
  }
})
