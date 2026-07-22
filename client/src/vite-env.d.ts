/// <reference types="vite/client" />

// Injetado em build time pelo `define` do vite.config.ts (GITHUB_SHA no CI,
// fallback dev-<timestamp> local). Usado na URL versionada do service worker.
declare const __BUILD_ID__: string;
