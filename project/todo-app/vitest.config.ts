import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './vitest.setup.ts',
    include: ['ts/tests/**/*.test.ts'],
    coverage: {
      reporter: ['text', ['html', { subDir: 'coverage', type: 'lcov' }]],
      exclude: ['ts/tests/**', 'ts/types/**'],
      provider : 'v8',
      enabled: true,
    },
  }
});
