import { defineConfig } from "orval";

export default defineConfig({
  penDrive: {
    input: "../backend/docs/openapi/swagger.json",
    output: {
      client: "fetch",
      mode: "split",
      target: "src/lib/api/generated/client.ts",
      schemas: "src/lib/api/generated/model",
      clean: true,
      override: {
        mutator: {
          path: "src/lib/api/http.ts",
          name: "customFetch",
        },
      },
    },
  },
});
