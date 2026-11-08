import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    allowedHosts: ['dozens-years-alternatively-laboratory.trycloudflare.com', 'blah-struct-knowledge-netscape.trycloudflare.com'],
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
      '/solana-rpc': {
        target: 'https://api.devnet.solana.com',
        changeOrigin: true,
        rewrite: () => '/',
      },
    },
  },
});
