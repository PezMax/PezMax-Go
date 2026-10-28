import { defineConfig } from '@vben/vite-config';

export default defineConfig(async () => {
  return {
    application: {},
    vite: {
      server: {
        proxy: {
          '/api': {
            changeOrigin: true,
            // Go 服务入口，main.go 默认监听 9033，并在 /api 下注册 Vben 接口。
            target: 'http://127.0.0.1:9033',
            ws: true,
          },
        },
      },
    },
  };
});
