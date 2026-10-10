import { expect, test, vi } from "vitest";
import { listIncidents } from "./search-incidents";

test("incident history uses typed paging and optional status filters", async () => {
	const response = { data: [], pagination: { page: 1, limit: 20, total: 0, total_pages: 0 } };
	const request = vi.fn().mockResolvedValue(response);
	expect(await listIncidents(request)).toEqual(response);
	expect(request).toHaveBeenLastCalledWith("GET", "/admin/search/incidents", undefined, {
		page: 1,
		limit: 20,
	});
	await listIncidents(request, { page: 2, limit: 10, status: "resolved" });
	expect(request).toHaveBeenLastCalledWith("GET", "/admin/search/incidents", undefined, {
		page: 2,
		limit: 10,
		status: "resolved",
	});
});
