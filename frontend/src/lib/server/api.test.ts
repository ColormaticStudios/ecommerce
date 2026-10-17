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
		getClientAddress: () => "198.51.100.7",
		request: new Request("http://127.0.0.1:4173/admin/cms", {
			headers: {
				cookie: "session_token=test-session; csrf_token=test-csrf",
			},
		}),
		fetch: async () => {
			throw new Error("event.fetch should not be used for external API requests");
		},
	};

	const result = await serverRequest<{ ok: boolean }>(event, "/content/navigation/header", {
		page: 2,
		filters: { role: "admin" },
	});

	expect(result).toEqual({ ok: true });
	expect(calls).toHaveLength(1);
	expect(calls[0]?.input).toBe(
		"http://localhost:3000/api/v1/content/navigation/header?page=2&filters%5Brole%5D=admin"
	);

	const headers = calls[0]?.init?.headers as Headers;
	expect(headers.get("content-type")).toBe("application/json");
	expect(headers.get("cookie")).toBe("session_token=test-session; csrf_token=test-csrf");
});

test("encodes nested attribute arrays with indices while keeping top-level filters repeated", async () => {
	const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("{}"));
	await serverRequest(
		{ request: new Request("http://localhost/search"), getClientAddress: () => "198.51.100.8" },
		"/search/products",
		{
			attribute: { color: ["red", "blue"], waterproof: ["false"], weight: ["0"] },
			brand_slug: ["north", "south"],
			category_slug: ["bags", "shoes"],
		}
	);
	const url = new URL(String(fetchMock.mock.calls[0]?.[0]));
	expect(url.searchParams.get("attribute[color][0]")).toBe("red");
	expect(url.searchParams.get("attribute[color][1]")).toBe("blue");
	expect(url.searchParams.get("attribute[waterproof][0]")).toBe("false");
	expect(url.searchParams.get("attribute[weight][0]")).toBe("0");
	expect(url.searchParams.has("attribute[color]")).toBe(false);
	expect(url.searchParams.getAll("brand_slug")).toEqual(["north", "south"]);
	expect(url.searchParams.getAll("category_slug")).toEqual(["bags", "shoes"]);
});

test("SSR callers forward their resolved addresses instead of spoofed headers", async () => {
	const calls: RequestInit[] = [];
	vi.spyOn(globalThis, "fetch").mockImplementation(async (_, init) => {
		calls.push(init!);
		return new Response("{}");
	});
	for (const address of ["198.51.100.1", "198.51.100.2"]) {
		await serverRequest(
			{
				request: new Request("http://localhost/search", {
					headers: { "X-Forwarded-For": "203.0.113.99" },
				}),
				getClientAddress: () => address,
			},
			"/search/products"
		);
	}
	expect(calls.map((call) => new Headers(call.headers).get("X-Forwarded-For"))).toEqual([
		"198.51.100.1",
		"198.51.100.2",
	]);
});

test("canonical URL locale and blank market override conflicting preferences in SSR", async () => {
	const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("{}"));
	await serverRequest(
		{
			request: new Request("https://shop.test/search?locale=fr&market=", {
				headers: {
					cookie: "locale=en-US; market=CA",
					"Accept-Language": "en-US",
					"X-Locale": "en-US",
					"X-Market": "US",
				},
			}),
			getClientAddress: () => "198.51.100.1",
		},
		"/search/products"
	);
	const url = new URL(String(fetchMock.mock.calls[0][0]));
	const headers = new Headers(fetchMock.mock.calls[0][1]?.headers);
	expect(url.searchParams.get("locale")).toBe("fr");
	expect(url.searchParams.get("market")).toBe("");
	expect(url.searchParams.has("market")).toBe(true);
	for (const header of ["Accept-Language", "X-Locale", "X-Market"])
		expect(headers.has(header)).toBe(false);
});
