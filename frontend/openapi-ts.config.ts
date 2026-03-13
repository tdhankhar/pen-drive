import { defineConfig } from '@hey-api/openapi-ts';

export default defineConfig({
  input: '../backend/docs/openapi/swagger.json',
  output: {
    path: 'src/lib/api/generated',
    format: 'prettier',
  },
  plugins: [
    '@hey-api/client-fetch',
    '@hey-api/schemas',
    '@hey-api/sdk',
    {
      name: '@hey-api/typescript',
      enums: 'javascript',
    },
    '@tanstack/react-query',
  ],
});
