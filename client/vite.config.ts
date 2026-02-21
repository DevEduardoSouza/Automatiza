import path from 'node:path';
import react from '@vitejs/plugin-react';
import { sentryVitePlugin } from '@sentry/vite-plugin';
import { defineConfig } from 'vite';

const ReactCompilerConfig = {
  target: '19',
};

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => ({
  server: {
    host: '::',
    port: 5173, // Porta padrao do Vite (8080 estava em conflito com Evolution API)
    proxy: {
      // Proxy para o backend Go - resolve problema de cookies cross-origin
      '/connect': {
        target: 'http://127.0.0.1:3000',
        changeOrigin: true,
        // Garantir que cookies sao encaminhados
        cookieDomainRewrite: 'localhost',
        secure: false,
      },
      '/api': {
        target: 'http://127.0.0.1:3000',
        changeOrigin: true,
        cookieDomainRewrite: 'localhost',
        secure: false,
      },
    },
  },
  plugins: [
    react({
      babel: {
        plugins: [['babel-plugin-react-compiler', ReactCompilerConfig]],
      },
    }),
    // Sentry source map upload (production builds only)
    mode === 'production' &&
      sentryVitePlugin({
        org: process.env.SENTRY_ORG,
        project: process.env.SENTRY_PROJECT,
        authToken: process.env.SENTRY_AUTH_TOKEN,
        release: {
          name: process.env.SENTRY_RELEASE,
        },
        sourcemaps: {
          filesToDeleteAfterUpload: ['./dist/**/*.map'],
        },
        telemetry: false,
      }),
  ].filter(Boolean),
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    sourcemap: 'hidden',
    rollupOptions: {
      output: {
        manualChunks: {
          // Core React ecosystem - loaded on every page
          'vendor-react': ['react', 'react-dom', 'react-router'],

          // UI primitives - Radix UI components
          'vendor-radix': [
            '@radix-ui/react-accordion',
            '@radix-ui/react-alert-dialog',
            '@radix-ui/react-aspect-ratio',
            '@radix-ui/react-avatar',
            '@radix-ui/react-checkbox',
            '@radix-ui/react-collapsible',
            '@radix-ui/react-context-menu',
            '@radix-ui/react-dialog',
            '@radix-ui/react-dropdown-menu',
            '@radix-ui/react-hover-card',
            '@radix-ui/react-label',
            '@radix-ui/react-menubar',
            '@radix-ui/react-navigation-menu',
            '@radix-ui/react-popover',
            '@radix-ui/react-progress',
            '@radix-ui/react-radio-group',
            '@radix-ui/react-scroll-area',
            '@radix-ui/react-select',
            '@radix-ui/react-separator',
            '@radix-ui/react-slider',
            '@radix-ui/react-slot',
            '@radix-ui/react-switch',
            '@radix-ui/react-tabs',
            '@radix-ui/react-toast',
            '@radix-ui/react-toggle',
            '@radix-ui/react-toggle-group',
            '@radix-ui/react-tooltip',
          ],

          // Data fetching and state management
          'vendor-query': [
            '@tanstack/react-query',
            '@connectrpc/connect-query',
            '@connectrpc/connect',
            '@connectrpc/connect-web',
          ],

          // Protobuf/gRPC
          'vendor-protobuf': ['@bufbuild/protobuf', '@bufbuild/protovalidate'],

          // Forms
          'vendor-forms': ['react-hook-form', '@hookform/resolvers', 'zod'],

          // Charts - only needed on dashboard/analytics pages
          'vendor-charts': ['recharts'],

          // Date utilities
          'vendor-date': ['date-fns', 'react-day-picker'],

          // Drag and drop
          'vendor-dnd': ['@dnd-kit/core', '@dnd-kit/sortable', '@dnd-kit/utilities'],

          // Internationalization
          'vendor-i18n': ['i18next', 'react-i18next', 'i18next-browser-languagedetector'],

          // Other utilities
          'vendor-utils': [
            'axios',
            'class-variance-authority',
            'clsx',
            'tailwind-merge',
            'lucide-react',
          ],
        },
      },
    },
  },
}));
