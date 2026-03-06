import { defineConfig, devices } from "@playwright/test";

// Test-only Playwright configuration for frontend/backend E2E coverage.
// This uses a dedicated test API server (`cmd/e2e-server`) and should not be
// used for production runtime configuration.
const apiPort = process.env.E2E_API_PORT || "3001";
const appPort = process.env.E2E_APP_PORT || "4173";
const isCI = Boolean(process.env.CI);
const dbDriver = (process.env.E2E_DB_DRIVER || (isCI ? "postgres" : "sqlite")).toLowerCase();
const dbPath = process.env.E2E_DB_PATH || "/tmp/ecommerce-e2e.sqlite";
const dbURL = process.env.E2E_DB_URL;
const apiBaseURL = `http://127.0.0.1:${apiPort}`;
const appBaseURL = `http://127.0.0.1:${appPort}`;

if (dbDriver !== "postgres" && dbDriver !== "sqlite") {
	throw new Error(`Unsupported E2E_DB_DRIVER=${dbDriver}. Expected "postgres" or "sqlite".`);
}

if (dbDriver === "postgres" && !dbURL) {
	throw new Error("E2E_DB_URL is required when E2E_DB_DRIVER=postgres.");
}

if (isCI && dbDriver !== "postgres") {
	throw new Error(
		"CI must run migration-sensitive E2E against Postgres (set E2E_DB_DRIVER=postgres)."
	);
}

const e2eServerEnv: Record<string, string> = {
	...process.env,
	E2E_API_PORT: apiPort,
	E2E_DB_DRIVER: dbDriver,
};

if (dbDriver === "postgres") {
	e2eServerEnv.E2E_DB_URL = dbURL!;
} else {
	e2eServerEnv.E2E_DB_PATH = dbPath;
}

export default defineConfig({
	testDir: "./e2e",
	fullyParallel: false,
	retries: process.env.CI ? 2 : 0,
	workers: process.env.CI ? 1 : undefined,
	reporter: "list",
	use: {
		baseURL: appBaseURL,
		trace: "on-first-retry",
	},
	projects: [
		{
			name: "chromium",
			use: { ...devices["Desktop Chrome"] },
		},
	],
	webServer: [
		{
			command: "cd .. && GOCACHE=/tmp/go-build go run ./cmd/e2e-server",
			env: e2eServerEnv,
			url: `${apiBaseURL}/__test/summary`,
			reuseExistingServer: !process.env.CI,
			timeout: 120_000,
		},
		{
			command: `PUBLIC_API_BASE_URL=${apiBaseURL} bun run dev --host 127.0.0.1 --port ${appPort}`,
			url: `${appBaseURL}/`,
			reuseExistingServer: !process.env.CI,
			timeout: 120_000,
		},
	],
});
