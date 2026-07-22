import path from 'node:path';
import react from '@vitejs/plugin-react';
import { sentryVitePlugin } from '@sentry/vite-plugin';
import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';

const ReactCompilerConfig = {
  target: '19',
};

// ID único por build (GITHUB_SHA no CI; timestamp local). Vai na URL do SW
// (/sw.js?v=BUILD_ID) para forçar MISS no edge Cloudflare a cada deploy.
const BUILD_ID = process.env.GITHUB_SHA?.slice(0, 12) ?? `dev-${Date.now()}`;

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => ({
  define: {
    __BUILD_ID__: JSON.stringify(BUILD_ID),
  },
  server: {
    host: '::',
    port: 3003,
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
    // PWA: service worker custom (injectManifest) + kill switch (selfDestroying).
    // manifest:false → mantém o manifest.webmanifest estático (identidade intocada).
    // injectRegister:false → registro manual com URL versionada (src/pwa/register.ts).
    VitePWA({
      strategies: 'injectManifest',
      srcDir: 'src/pwa',
      filename: 'sw.ts',
      injectRegister: false,
      registerType: 'prompt',
      selfDestroying: !!process.env.PWA_SELF_DESTROY,
      manifest: false,
      injectManifest: {
        globPatterns: ['**/*.{js,css,html,woff2,png,svg,ico}'],
        globIgnores: ['templates/**'],
        maximumFileSizeToCacheInBytes: 4 * 1024 * 1024,
      },
      devOptions: { enabled: false },
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
