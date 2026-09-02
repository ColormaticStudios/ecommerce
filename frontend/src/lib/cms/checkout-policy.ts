import type { CmsContentBlock } from "$lib/cms";

function isCheckoutURL(url: string): boolean {
	try {
		const parsed = new URL(url, "https://storefront.invalid");
		if (parsed.origin !== "https://storefront.invalid") return false;

		const decodedPath = decodeURIComponent(parsed.pathname);
		const normalizedPath = decodedPath.length > 1 ? decodedPath.replace(/\/+$/, "") : decodedPath;
		return normalizedPath === "/checkout";
	} catch {
		return false;
	}
}

export function applyCmsCheckoutPolicy(
	blocks: CmsContentBlock[],
	checkoutAllowed: boolean
): CmsContentBlock[] {
	if (checkoutAllowed) return blocks;

	return blocks.flatMap((block): CmsContentBlock[] => {
		if (block.type === "cta" && isCheckoutURL(block.url)) return [];
		if (block.type === "hero" && block.primary_cta && isCheckoutURL(block.primary_cta.url)) {
			return [{ ...block, primary_cta: undefined }];
		}
		if (
			(block.type === "promo_banner" || block.type === "promotion_highlight") &&
			block.link &&
			isCheckoutURL(block.link.url)
		) {
			return [{ ...block, link: undefined }];
		}
		if (block.type === "footer") {
			return [
				{
					...block,
					columns: block.columns.map((column) => ({
						...column,
						links: column.links.filter((link) => !isCheckoutURL(link.url)),
					})),
					social_links: block.social_links.filter((link) => !isCheckoutURL(link.url)),
				},
			];
		}
		return [block];
	});
}
