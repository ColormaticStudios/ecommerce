import { afterEach, expect, test, vi } from "vitest";
import { API } from "./api";

afterEach(() => {
	vi.restoreAllMocks();
	vi.unstubAllGlobals();
});

test("click telemetry uses keepalive while retaining session credentials and CSRF", async () => {
	vi.stubGlobal("document", { cookie: "csrf_token=search-csrf" });
	const fetch = vi
		.spyOn(globalThis, "fetch")
		.mockResolvedValue(
			new Response(JSON.stringify({ click_id: "verified-click" }), {
				headers: { "Content-Type": "application/json" },
			})
		);
	const body = {
		session_token: "session",
		impression_id: "impression",
		event_id: "event",
		type: "click" as const,
		product_id: 1,
		position: 1,
	};
	await expect(new API("https://api.example.test").recordSearchEvent(body)).resolves.toEqual({
		click_id: "verified-click",
	});
	const [url, init] = fetch.mock.calls[0];
	expect(String(url)).toBe("https://api.example.test/api/v1/search/events");
	expect(init).toMatchObject({
		method: "POST",
		keepalive: true,
		credentials: "include",
		body: JSON.stringify(body),
	});
	expect(new Headers(init?.headers).get("X-CSRF-Token")).toBe("search-csrf");
});

test("dashboard sends bounded paging independently of its global reporting period", async () => {
	const fetch = vi
		.spyOn(globalThis, "fetch")
		.mockResolvedValue(new Response("{}", { headers: { "Content-Type": "application/json" } }));
	await new API("https://api.example.test").adminSearchAnalytics(30, 2, 20);
	const url = new URL(String(fetch.mock.calls[0][0]));
	expect(Object.fromEntries(url.searchParams)).toEqual({ days: "30", page: "2", limit: "20" });
});
