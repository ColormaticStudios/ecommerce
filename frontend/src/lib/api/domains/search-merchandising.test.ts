import { afterEach, expect, test, vi } from "vitest";
import { API } from "$lib/api";
import { ApiProblemError } from "$lib/api/errors";
afterEach(() => vi.restoreAllMocks());
test("preview and audit use shared API transport with generated payload shapes", async () => {
	const requests: Array<{ url: string; init?: RequestInit }> = [];
	vi.spyOn(globalThis, "fetch").mockImplementation(async (url, init) => {
		requests.push({ url: String(url), init });
		return new Response(
			JSON.stringify(
				String(url).endsWith("/preview") ? { baseline: {}, proposed: {} } : { data: [] }
			)
		);
	});
	const api = new API("http://localhost");
	const body = { filters: { q: "jacket", page: 1, limit: 20 }, rules: [] };
	await api.previewAdminSearch(body);
	await api.listAdminSearchMerchandisingAudit(7);
	expect(await api.listAllAdminSearchMerchandisingAudit()).toEqual([]);
	expect(requests[0].url).toBe("http://localhost/api/v1/admin/search/preview");
	expect(requests[0].init?.credentials).toBe("include");
	expect(JSON.parse(String(requests[0].init?.body))).toEqual(body);
	expect(requests[1].url).toBe("http://localhost/api/v1/admin/search/merchandising-rules/7/audit");
	expect(requests[2].url).toBe("http://localhost/api/v1/admin/search/merchandising-audit");
});
test("configuration conflicts propagate as structured problems", async () => {
	vi.spyOn(globalThis, "fetch").mockResolvedValue(
		new Response(
			JSON.stringify({
				error_code: "search_configuration_conflict",
				detail: "Name already exists.",
			}),
			{ status: 409 }
		)
	);
	const api = new API("http://localhost");
	await expect(
		api.updateAdminSearchMerchandisingRule(7, { is_active: false })
	).rejects.toBeInstanceOf(ApiProblemError);
});
