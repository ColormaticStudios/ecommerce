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
const verboseWebServerLogs = Boolean(process.env.E2E_VERBOSE_LOGS);

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

function withOptionalLogRedirect(command: string, logPath: string): string {
	if (verboseWebServerLogs) {
		return command;
	}
	return `${command} >${logPath} 2>&1`;
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
			command: withOptionalLogRedirect(
				"cd .. && GOCACHE=/tmp/go-build go run ./cmd/e2e-server",
				"/tmp/ecommerce-e2e-api.log"
			),
			env: e2eServerEnv,
			url: `${apiBaseURL}/__test/summary`,
			reuseExistingServer: !process.env.CI,
			timeout: 120_000,
		},
		{
			command: withOptionalLogRedirect(
				`PUBLIC_API_BASE_URL=${apiBaseURL} bun run dev --host 127.0.0.1 --port ${appPort}`,
				"/tmp/ecommerce-e2e-frontend.log"
			),
			url: `${appBaseURL}/`,
			reuseExistingServer: !process.env.CI,
			timeout: 120_000,
		},
	],
});
