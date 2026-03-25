import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import obfuscatorPlugin from 'rollup-plugin-obfuscator'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
    minify: 'terser',
    chunkSizeWarningLimit: 500, // KB
    terserOptions: {
      compress: {
        drop_console: true,  // Remove console.log
        drop_debugger: true, // Remove debugger statements
      },
      mangle: {
        toplevel: true,
      },
    },
    rollupOptions: {
      output: {
        // Manual chunks for better caching and smaller initial load
        manualChunks: {
          // React core (changes rarely)
          'react-vendor': ['react', 'react-dom', 'react-router-dom'],
          // Data fetching/state
          'data-vendor': ['@tanstack/react-query', 'zustand', 'axios'],
          // UI libraries
          'ui-vendor': ['@heroicons/react', 'react-hot-toast', 'clsx'],
          // Table/grid (heavy)
          'table-vendor': ['@tanstack/react-table'],
          // Charts are lazy-loaded — no manual chunk needed
        },
      },
      plugins: [
        obfuscatorPlugin({
          options: {
            compact: true,
            controlFlowFlattening: true,
            controlFlowFlatteningThreshold: 0.75,
            deadCodeInjection: true,
            deadCodeInjectionThreshold: 0.4,
            debugProtection: false,
            disableConsoleOutput: true,
            identifierNamesGenerator: 'hexadecimal',
            renameGlobals: false,
            rotateStringArray: true,
            selfDefending: true,
            shuffleStringArray: true,
            splitStrings: true,
            splitStringsChunkLength: 10,
            stringArray: true,
            stringArrayEncoding: ['base64'],
            stringArrayThreshold: 0.75,
            transformObjectKeys: true,
            unicodeEscapeSequence: false,
          },
        }),
      ],
    },
  },
})
