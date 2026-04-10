import assert from "node:assert/strict";
import test from "node:test";
import {
	createPublicRuntimeEnvScript,
	resolvePublicApiBaseUrl,
	serializePublicRuntimeEnv,
} from "./public-env";

test("resolvePublicApiBaseUrl prefers runtime server env over client env", () => {
	const baseUrl = resolvePublicApiBaseUrl({
		serverEnv: {
			PUBLIC_API_BASE_URL: "https://server.example/",
		},
		clientEnv: {
			PUBLIC_API_BASE_URL: "https://client.example/",
		},
	});

	assert.equal(baseUrl, "https://server.example/");
});

test("resolvePublicApiBaseUrl falls back to the client runtime env", () => {
	const baseUrl = resolvePublicApiBaseUrl({
		clientEnv: {
			STORYBOOK_PUBLIC_API_BASE_URL: "https://storybook.example/",
		},
	});

	assert.equal(baseUrl, "https://storybook.example/");
});

test("resolvePublicApiBaseUrl uses the default when no runtime env is configured", () => {
	assert.equal(resolvePublicApiBaseUrl(), "http://localhost:3000");
});

test("serializePublicRuntimeEnv escapes script-sensitive characters", () => {
	const serialized = serializePublicRuntimeEnv({
		PUBLIC_API_BASE_URL: "https://example.com/</script>",
	});

	assert.equal(serialized.includes("</script>"), false);
	assert.equal(serialized.includes("\\u003c/script>"), true);
});

test("createPublicRuntimeEnvScript seeds the shared runtime global", () => {
	const script = createPublicRuntimeEnvScript({
		PUBLIC_API_BASE_URL: "https://runtime.example/",
	});

	assert.equal(
		script,
		'<script>globalThis.__PUBLIC_ENV__ = Object.freeze({"PUBLIC_API_BASE_URL":"https://runtime.example/"});</script>'
	);
});
