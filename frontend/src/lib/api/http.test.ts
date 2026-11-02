import { expect, it } from "vitest";
import { appendQueryParams } from "./http";

it("indexes nested filter arrays while retaining repeated top-level parameters", () => {
	const url = new URL("https://example.test/api/v1/search/products");
	appendQueryParams(url, {
		attribute: { color: ["red", "blue"], waterproof: [false], weight: [0] },
		brand_slug: ["one", "two"],
		q: "",
		missing: null,
	});
	expect(url.searchParams.get("attribute[color][0]")).toBe("red");
	expect(url.searchParams.get("attribute[color][1]")).toBe("blue");
	expect(url.searchParams.get("attribute[waterproof][0]")).toBe("false");
	expect(url.searchParams.get("attribute[weight][0]")).toBe("0");
	expect(url.searchParams.has("attribute[color]")).toBe(false);
	expect(url.searchParams.getAll("brand_slug")).toEqual(["one", "two"]);
	expect(url.searchParams.has("q")).toBe(false);
	expect(url.searchParams.has("missing")).toBe(false);
});
