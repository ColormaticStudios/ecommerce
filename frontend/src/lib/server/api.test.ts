import assert from "node:assert/strict";
import test from "node:test";
import { serverRequest } from "./api";

test("serverRequest uses runtime fetch and forwards cookies/query params", async () => {
	const originalFetch = globalThis.fetch;
	const calls: Array<{ input: string; init?: RequestInit }> = [];

	globalThis.fetch = (async (input: string | URL | Request, init?: RequestInit) => {
		calls.push({ input: String(input), init });
		return new Response(JSON.stringify({ ok: true }), {
			status: 200,
			headers: { "Content-Type": "application/json" },
		});
	}) as typeof fetch;

	try {
		const event = {
			request: new Request("http://127.0.0.1:4173/admin/storefront", {
				headers: {
					cookie: "session_token=test-session; csrf_token=test-csrf",
				},
			}),
			fetch: async () => {
				throw new Error("event.fetch should not be used for external API requests");
			},
		};

		const result = await serverRequest<{ ok: boolean }>(event, "/storefront", {
			page: 2,
			filters: { role: "admin" },
		});

		assert.deepEqual(result, { ok: true });
		assert.equal(calls.length, 1);
		assert.equal(
			calls[0]?.input,
			"http://localhost:3000/api/v1/storefront?page=2&filters%5Brole%5D=admin"
		);

		const headers = calls[0]?.init?.headers as Headers;
		assert.equal(headers.get("content-type"), "application/json");
		assert.equal(headers.get("cookie"), "session_token=test-session; csrf_token=test-csrf");
	} finally {
		globalThis.fetch = originalFetch;
	}
});
