import DefaultTheme from 'vitepress/theme'
import { onMounted, watch, nextTick } from 'vue'
import { useRoute } from 'vitepress'
import mediumZoom from 'medium-zoom'

import './custom.css'

export default {
  ...DefaultTheme,
  enhanceApp({ app, router, siteData }) {
    // app is the Vue 3 app instance from `createApp()`. router is VitePress'
    // custom router. `siteData` is a `ref` of current site-level metadata.
  },
  setup() {
    const route = useRoute()
    
    const initializeZoom = () => {
      // Initialize zoom for regular images
      mediumZoom('.main img', { 
        background: 'var(--vp-c-bg)',
        margin: 24,
        scrollOffset: 40
      })
      
      // Initialize zoom for Mermaid diagrams (SVGs)
      mediumZoom('.main .mermaid svg', {
        background: 'var(--vp-c-bg)',
        margin: 24,
        scrollOffset: 40
      })
      
      // Initialize zoom for any other diagrams or code block images
      mediumZoom('.main pre img, .main .diagram img', {
        background: 'var(--vp-c-bg)',
        margin: 24,
        scrollOffset: 40
      })
    }
    
    onMounted(() => {
      // Initial setup
      initializeZoom()
      
      // Re-initialize when Mermaid diagrams are rendered
      const observer = new MutationObserver((mutations) => {
        mutations.forEach((mutation) => {
          if (mutation.addedNodes.length > 0) {
            const hasMermaid = Array.from(mutation.addedNodes).some(node => 
              node.nodeType === 1 && (
                node.classList?.contains('mermaid') || 
                node.querySelector?.('.mermaid')
              )
            )
            if (hasMermaid) {
              setTimeout(initializeZoom, 100) // Small delay for Mermaid rendering
            }
          }
        })
      })
      
      observer.observe(document.body, {
        childList: true,
        subtree: true
      })
    })
    
    watch(
      () => route.path,
      () => nextTick(() => {
        setTimeout(initializeZoom, 200) // Delay for page content to load
      })
    )
  }
}