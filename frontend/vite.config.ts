import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  build: {
    // 保留較保守的 JavaScript 輸出目標，涵蓋 macOS 13 以上的 WebView。
    target: 'safari15',
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('/src/i18n/')) {
            return 'i18n';
          }
          if (!id.includes('node_modules')) {
            return undefined;
          }
          if (id.includes('/react/') || id.includes('/react-dom/') || id.includes('/scheduler/')) {
            return 'react-vendor';
          }
          if (id.includes('/xterm/') || id.includes('/@xterm/')) {
            return 'terminal-vendor';
          }
          if (id.includes('/@fortawesome/')) {
            return 'icons-vendor';
          }
          return 'vendor';
        },
      },
    },
  },
});
