import type { PageServerLoad } from "./$types";
import { API } from "$lib/api";
import type { ProductModel } from "$lib/models";
import { setPublicPageCacheHeaders } from "$lib/server/cache";
import type {
	StorefrontHomepageSectionModel,
	StorefrontProductSectionModel,
} from "$lib/storefront";
import { STOREFRONT_LIMITS } from "$lib/storefront";

interface HomepageSectionData extends StorefrontHomepageSectionModel {
	products: ProductModel[];
}

async function loadManualProducts(
	api: API,
	productIds: number[],
	limit: number
): Promise<ProductModel[]> {
	const ids = productIds.filter((id) => Number.isInteger(id) && id > 0).slice(0, limit);
	if (ids.length === 0) {
		return [];
	}

	const results = await Promise.allSettled(ids.map((id) => api.getProduct(id)));
	return results
		.filter(
			(result): result is PromiseFulfilledResult<ProductModel> => result.status === "fulfilled"
		)
		.map((result) => result.value);
}

async function loadProductSection(
	api: API,
	config: StorefrontProductSectionModel
): Promise<ProductModel[]> {
	const limit = Math.min(
		STOREFRONT_LIMITS.max_product_section_limit,
		Math.max(1, config.limit || STOREFRONT_LIMITS.default_product_section_limit)
	);
	if (config.source === "manual") {
		return loadManualProducts(api, config.product_ids, limit);
	}

	const page = await api.listProducts({
		q: config.source === "search" ? config.query.trim() || undefined : undefined,
		sort: config.sort,
		order: config.order,
		page: 1,
		limit,
	});
	return page.data;
}

export const load: PageServerLoad = async (event) => {
	setPublicPageCacheHeaders(event);
	const { parent } = event;
	const { storefront } = await parent();
	const api = new API();
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
			const products = await loadProductSection(api, section.product_section);
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
