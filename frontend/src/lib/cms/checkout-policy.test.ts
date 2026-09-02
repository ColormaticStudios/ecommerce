import { describe, expect, test } from "vitest";
import { applyCmsCheckoutPolicy } from "./checkout-policy";
import type { CmsContentBlock } from "$lib/cms";

describe("applyCmsCheckoutPolicy", () => {
	const blocks: CmsContentBlock[] = [
		{ type: "cta", label: "Checkout", url: "/checkout" },
		{ type: "cta", label: "Search", url: "/search" },
		{ type: "hero", title: "Hero", primary_cta: { label: "Pay", url: "/checkout?from=cms" } },
		{
			type: "promotion_highlight",
			title: "Offer",
			link: { label: "Redeem", url: "/checkout#payment" },
		},
	];

	test("preserves checkout actions for authenticated or guest-enabled rendering", () => {
		expect(applyCmsCheckoutPolicy(blocks, true)).toEqual(blocks);
	});

	test("removes checkout actions while retaining unrelated CMS content", () => {
		const result = applyCmsCheckoutPolicy(blocks, false);
		expect(result).toHaveLength(3);
		expect(result[0]).toMatchObject({ type: "cta", url: "/search" });
		expect(result[1]).toEqual({ type: "hero", title: "Hero", primary_cta: undefined });
		expect(result[2]).toEqual({ type: "promotion_highlight", title: "Offer", link: undefined });
	});

	test.each(["/checkout/", "/%63heckout", "/checkout%2F"])(
		"removes normalized checkout URL %s",
		(url) => {
			const result = applyCmsCheckoutPolicy([{ type: "cta", label: "Checkout", url }], false);
			expect(result).toEqual([]);
		}
	);

	test("does not remove checkout-looking links to another origin", () => {
		const block: CmsContentBlock = {
			type: "cta",
			label: "Partner checkout",
			url: "https://example.com/checkout",
		};
		expect(applyCmsCheckoutPolicy([block], false)).toEqual([block]);
	});
});
