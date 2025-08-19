import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  
  // Performance optimizations
  build: {
    // Enable minification
    minify: 'esbuild',
    
    // Optimize chunks
    rollupOptions: {
      output: {
        manualChunks: {
          // Separate vendor libraries for better caching
          vendor: ['react', 'react-dom'],
          analytics: ['./src/utils/analytics.ts', './src/hooks/useAnalytics.ts'],
        },
      },
    },
    
    // Target modern browsers for smaller bundles
    target: 'es2020',
    
    // Enable source maps for debugging (but keep them small)
    sourcemap: false, // Disable in production for smaller size
    
    // Optimize CSS
    cssCodeSplit: true,
  },
  
  // Optimize dependencies
  optimizeDeps: {
    include: ['react', 'react-dom'],
  },
  
  // Preload optimizations
  server: {
    preTransformRequests: false, // Only transform on demand
  },
})
