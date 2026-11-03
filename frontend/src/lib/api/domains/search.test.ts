import { expect, test, vi } from "vitest";
import { listRankingProfiles, listSynonyms, listTypoProfiles } from "./search";
test("search configuration lists use the existing typed admin resources", async () => {
	const request = vi.fn().mockResolvedValue({ data: [{ id: 1, name: "Default" }] });
	expect(await listSynonyms(request)).toEqual([{ id: 1, name: "Default" }]);
	await listTypoProfiles(request);
	await listRankingProfiles(request);
	expect(request.mock.calls.map((call) => call.slice(0, 2))).toEqual([
		["GET", "/admin/search/synonyms"],
		["GET", "/admin/search/typo-profiles"],
		["GET", "/admin/search/ranking-profiles"],
	]);
});
