import config from "./web/playwright.config";

export default {
  ...config,
  testDir: "web/tests/e2e",
};
