import { afterEach, expect, test, vi } from "vitest";
import { serverRequest } from "./api";

afterEach(() => {
	vi.restoreAllMocks();
});

test("serverRequest uses runtime fetch and forwards cookies/query params", async () => {
	const calls: Array<{ input: string; init?: RequestInit }> = [];

	vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
		calls.push({ input: String(input), init });
		return new Response(JSON.stringify({ ok: true }), {
			status: 200,
			headers: { "Content-Type": "application/json" },
		});
	});

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

	expect(result).toEqual({ ok: true });
	expect(calls).toHaveLength(1);
	expect(calls[0]?.input).toBe(
		"http://localhost:3000/api/v1/storefront?page=2&filters%5Brole%5D=admin"
	);

	const headers = calls[0]?.init?.headers as Headers;
	expect(headers.get("content-type")).toBe("application/json");
	expect(headers.get("cookie")).toBe("session_token=test-session; csrf_token=test-csrf");
});
