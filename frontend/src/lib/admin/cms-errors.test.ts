import { describe, expect, it } from "vitest";
import { formatCmsAdminError } from "./cms-errors";

describe("formatCmsAdminError", () => {
	it("turns block field paths into one-based editor messages", () => {
		expect(
			formatCmsAdminError(
				{ body: { error: "invalid cms page: payload.blocks[3].body is required" } },
				"Unable to save CMS draft."
			)
		).toBe("Block #4 is missing body text.");
	});

	it("describes nested links and gallery images", () => {
		expect(
			formatCmsAdminError(
				{ body: { error: "invalid cms page: payload.blocks[1].link.url is required" } },
				"fallback"
			)
		).toBe("Block #2 is missing link URL.");
		expect(
			formatCmsAdminError(
				{ body: { error: "invalid cms page: payload.blocks[2].images[1].media_id is required" } },
				"fallback"
			)
		).toBe("Block #3, image #2 is missing image.");
	});

	it("formats ranges and non-block CMS validation", () => {
		expect(
			formatCmsAdminError(
				{ body: { error: "invalid cms page: payload.blocks[0].rating must be between 1 and 5" } },
				"fallback"
			)
		).toBe("Block #1 has an invalid rating. It must be between 1 and 5.");
		expect(formatCmsAdminError("invalid cms page: title is required", "fallback")).toBe(
			"Title is required."
		);
	});
});
