import type { PageServerLoad } from "./$types";
import { parseProduct, type ProductModel } from "$lib/models";
import { setPublicPageCacheHeaders } from "$lib/server/cache";
import type {
	StorefrontHomepageSectionModel,
	StorefrontProductSectionModel,
} from "$lib/storefront";
import { STOREFRONT_LIMITS } from "$lib/storefront";
import { serverRequest } from "$lib/server/api";
import type { components } from "$lib/api/generated/openapi";

type ProductPagePayload = components["schemas"]["ProductPage"];
type ProductPayload = components["schemas"]["Product"];

interface HomepageSectionData extends StorefrontHomepageSectionModel {
	products: ProductModel[];
}

async function loadManualProducts(
	event: Parameters<PageServerLoad>[0],
	productIds: number[],
	limit: number
): Promise<ProductModel[]> {
	const ids = productIds.filter((id) => Number.isInteger(id) && id > 0).slice(0, limit);
	if (ids.length === 0) {
		return [];
	}

	const results = await Promise.allSettled(
		ids.map((id) => serverRequest<ProductPayload>(event, `/products/${id}`))
	);
	return results
		.filter(
			(result): result is PromiseFulfilledResult<ProductPayload> => result.status === "fulfilled"
		)
		.map((result) => parseProduct(result.value));
}

async function loadProductSection(
	event: Parameters<PageServerLoad>[0],
	config: StorefrontProductSectionModel
): Promise<ProductModel[]> {
	const limit = Math.min(
		STOREFRONT_LIMITS.max_product_section_limit,
		Math.max(1, config.limit || STOREFRONT_LIMITS.default_product_section_limit)
	);
	if (config.source === "manual") {
		return loadManualProducts(event, config.product_ids, limit);
	}

	const page = await serverRequest<ProductPagePayload>(event, "/products", {
		q: config.source === "search" ? config.query.trim() || undefined : undefined,
		sort: config.sort,
		order: config.order,
		page: 1,
		limit,
	});
	return page.data.map(parseProduct);
}

export const load: PageServerLoad = async (event) => {
	setPublicPageCacheHeaders(event);
	const { parent } = event;
	const { storefront } = await parent();
	let errorMessage = "";

	const homepageSections: HomepageSectionData[] = [];
	for (const section of storefront.homepage_sections) {
		if (!section.enabled) {
			continue;
		}
		if (section.type !== "products" || !section.product_section) {
			homepageSections.push({ ...section, products: [] });
			continue;
		}

		try {
			const products = await loadProductSection(event, section.product_section);
			homepageSections.push({ ...section, products });
		} catch (err) {
			console.error(err);
			errorMessage = "Unable to load one or more homepage product sections.";
			homepageSections.push({ ...section, products: [] });
		}
	}

	return {
		storefront,
		errorMessage,
		homepageSections,
	};
};
