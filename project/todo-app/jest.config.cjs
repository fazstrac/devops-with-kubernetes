module.exports = {
  rootDir: '.',
  testEnvironment: 'jest-environment-jsdom',
  transform: {
    '^.+\\.tsx?$': ['ts-jest', { tsconfig: 'ts/tsconfig.jest.json' }]
  },
  testMatch: ['<rootDir>/ts/tests/**/*.test.ts', '<rootDir>/ts/tests/**/*.test.tsx'],
  setupFilesAfterEnv: ['<rootDir>/jest.setup.js'],
  moduleFileExtensions: ['ts', 'tsx', 'js', 'jsx', 'json', 'node']
};
