import { expect, test } from "vitest";
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

	expect(baseUrl).toBe("https://server.example/");
});

test("resolvePublicApiBaseUrl falls back to the client runtime env", () => {
	const baseUrl = resolvePublicApiBaseUrl({
		clientEnv: {
			STORYBOOK_PUBLIC_API_BASE_URL: "https://storybook.example/",
		},
	});

	expect(baseUrl).toBe("https://storybook.example/");
});

test("resolvePublicApiBaseUrl uses the default when no runtime env is configured", () => {
	expect(resolvePublicApiBaseUrl()).toBe("http://localhost:3000");
});

test("serializePublicRuntimeEnv escapes script-sensitive characters", () => {
	const serialized = serializePublicRuntimeEnv({
		PUBLIC_API_BASE_URL: "https://example.com/</script>",
	});

	expect(serialized.includes("</script>")).toBe(false);
	expect(serialized.includes("\\u003c/script>")).toBe(true);
});

test("createPublicRuntimeEnvScript seeds the shared runtime global", () => {
	const script = createPublicRuntimeEnvScript({
		PUBLIC_API_BASE_URL: "https://runtime.example/",
	});

	expect(script).toBe(
		'<script>globalThis.__PUBLIC_ENV__ = Object.freeze({"PUBLIC_API_BASE_URL":"https://runtime.example/"});</script>'
	);
});
