import { afterEach, expect, test, vi } from "vitest";
import { loadAuthConfig } from "./auth";

afterEach(() => {
	vi.restoreAllMocks();
});

test("loadAuthConfig returns backend auth config when request succeeds", async () => {
	vi.spyOn(globalThis, "fetch").mockResolvedValue(
		new Response(JSON.stringify({ local_sign_in_enabled: false, oidc_enabled: true }), {
			status: 200,
			headers: { "Content-Type": "application/json" },
		})
	);

	const result = await loadAuthConfig({
		request: new Request("http://127.0.0.1:4173/login"),
	});

	expect(result).toEqual({ local_sign_in_enabled: false, oidc_enabled: true });
});

test("loadAuthConfig falls back to local-only auth when the backend request fails", async () => {
	vi.spyOn(globalThis, "fetch").mockResolvedValue(
		new Response(JSON.stringify({ error: "boom" }), {
			status: 500,
			headers: { "Content-Type": "application/json" },
		})
	);
	const errorSpy = vi.spyOn(console, "error").mockImplementation(() => undefined);

	const result = await loadAuthConfig({
		request: new Request("http://127.0.0.1:4173/login"),
	});

	expect(result).toEqual({ local_sign_in_enabled: true, oidc_enabled: false });
	expect(errorSpy).toHaveBeenCalled();
});
