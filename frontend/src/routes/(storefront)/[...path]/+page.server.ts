import type { PageServerLoad } from "./$types";
import { error, redirect } from "@sveltejs/kit";
import { parseCmsPage, type CmsContentBlock, type CmsPageResponsePayload } from "$lib/cms";
import { parseCategory, parseProduct, type CategoryModel, type ProductModel } from "$lib/models";
import { serverRequest, type ServerAPIError } from "$lib/server/api";
import { setPublicPageCacheHeaders } from "$lib/server/cache";
import type { components } from "$lib/api/generated/openapi";

type ProductPagePayload = components["schemas"]["ProductPage"];
type ProductPayload = components["schemas"]["Product"];
type CategoryListPayload = components["schemas"]["CategoryListResponse"];
type RedirectResolution = components["schemas"]["CmsRedirectResolution"];

async function loadManualProducts(
	event: Parameters<PageServerLoad>[0],
	productIds: number[],
	limit: number
): Promise<ProductModel[]> {
	const ids = productIds.filter((id) => Number.isInteger(id) && id > 0).slice(0, limit);
	if (ids.length === 0) return [];
	const results = await Promise.allSettled(
		ids.map((id) => serverRequest<ProductPayload>(event, `/products/${id}`))
	);
	return results
		.filter(
			(result): result is PromiseFulfilledResult<ProductPayload> => result.status === "fulfilled"
		)
		.map((result) => parseProduct(result.value));
}

async function loadProductRail(
	event: Parameters<PageServerLoad>[0],
	block: Extract<CmsContentBlock, { type: "product_rail" }>
): Promise<ProductModel[]> {
	const limit = Math.min(24, Math.max(1, block.limit || 8));
	if (block.source === "manual") {
		return loadManualProducts(event, block.product_ids ?? [], limit);
	}
	const page = await serverRequest<ProductPagePayload>(event, "/products", {
		q: block.source === "search" ? block.query?.trim() || undefined : undefined,
		category_slug:
			block.source === "category" && block.category_slug?.trim()
				? [block.category_slug.trim()]
				: undefined,
		sort: block.sort ?? "created_at",
		order: block.order ?? "desc",
		page: 1,
		limit,
	});
	return page.data.map(parseProduct);
}

async function loadProductRails(
	event: Parameters<PageServerLoad>[0],
	blocks: CmsContentBlock[]
): Promise<Record<string, ProductModel[]>> {
	const rails: Record<string, ProductModel[]> = {};
	await Promise.all(
		blocks.map(async (block, index) => {
			if (block.type !== "product_rail") return;
			try {
				rails[`product_rail:${index}`] = await loadProductRail(event, block);
			} catch (err) {
				console.error("Failed to load CMS product rail", err);
				rails[`product_rail:${index}`] = [];
			}
		})
	);
	return rails;
}

async function loadCategoryTiles(
	event: Parameters<PageServerLoad>[0],
	blocks: CmsContentBlock[]
): Promise<Record<string, CategoryModel[]>> {
	const tileSets: Record<string, CategoryModel[]> = {};
	const categoryList = await serverRequest<CategoryListPayload>(event, "/categories");
	const bySlug = new Map(
		categoryList.data.map((category) => [category.slug, parseCategory(category)])
	);
	for (const [index, block] of blocks.entries()) {
		if (block.type !== "category_tiles") continue;
		tileSets[`category_tiles:${index}`] = block.category_slugs
			.map((slug) => bySlug.get(slug))
			.filter((category): category is CategoryModel => Boolean(category));
	}
	return tileSets;
}

async function loadInventoryProducts(
	event: Parameters<PageServerLoad>[0],
	blocks: CmsContentBlock[]
): Promise<Record<string, ProductModel | null>> {
	const products: Record<string, ProductModel | null> = {};
	await Promise.all(
		blocks.map(async (block, index) => {
			if (block.type !== "inventory_message") return;
			try {
				const product = await serverRequest<ProductPayload>(event, `/products/${block.product_id}`);
				products[`inventory_message:${index}`] = parseProduct(product);
			} catch (err) {
				console.error("Failed to load CMS inventory message product", err);
				products[`inventory_message:${index}`] = null;
			}
		})
	);
	return products;
}

export const load: PageServerLoad = async (event) => {
	const { params, parent, cookies, url } = event;
	const { draftPreview } = await parent();
	// SvelteKit concrete routes in this group keep precedence over this catch-all
	// route, so /cart, /checkout, /product/[id], and /search resolve before CMS pages.
	const routePath = params.path ?? "";
	const apiPath = routePath ? `/content/${encodeURIComponent(routePath)}` : "/content";
	try {
		const resolvedRedirect = await serverRequest<RedirectResolution>(event, "/content/redirect", {
			path: `/${routePath}`,
		});
		redirect(resolvedRedirect.redirect_type, resolvedRedirect.target_url);
	} catch (err) {
		const redirectError = err as ServerAPIError;
		if (redirectError.status !== 404) throw err;
	}
	let assignmentKey = cookies.get("cms_visitor");
	if (!assignmentKey) {
		assignmentKey = crypto.randomUUID();
		cookies.set("cms_visitor", assignmentKey, {
			path: "/",
			httpOnly: true,
			sameSite: "lax",
			secure: url.protocol === "https:",
			maxAge: 60 * 60 * 24 * 365,
		});
	}

	try {
		const explicitLocale = url.searchParams.get("locale")?.trim();
		if (explicitLocale) {
			cookies.set("storefront_locale", explicitLocale, {
				path: "/",
				httpOnly: false,
				sameSite: "lax",
				secure: url.protocol === "https:",
				maxAge: 60 * 60 * 24 * 365,
			});
		}
		const requestedLocale =
			explicitLocale ||
			cookies.get("storefront_locale") ||
			event.request.headers.get("accept-language")?.split(",")[0]?.split(";")[0]?.trim() ||
			undefined;
		const response = await serverRequest<CmsPageResponsePayload>(event, apiPath, {
			assignment_key: assignmentKey,
			market: url.searchParams.get("market") || undefined,
			locale: requestedLocale,
			segment: url.searchParams.get("segment") || undefined,
			utm_source: url.searchParams.get("utm_source") || undefined,
		});
		const page = parseCmsPage(response, Boolean(draftPreview?.active));
		if (draftPreview?.active) event.setHeaders({ "X-Robots-Tag": "noindex" });
		const [productRails, categoryTiles, inventoryProducts] = await Promise.all([
			loadProductRails(event, page.blocks),
			loadCategoryTiles(event, page.blocks).catch((err) => {
				console.error("Failed to load CMS category tiles", err);
				return {};
			}),
			loadInventoryProducts(event, page.blocks),
		]);
		setPublicPageCacheHeaders(event, true);
		return {
			page,
			productRails,
			categoryTiles,
			inventoryProducts,
			draftPreviewActive: Boolean(draftPreview?.active),
		};
	} catch (err) {
		const apiError = err as ServerAPIError;
		if (apiError.status === 404) {
			error(404, "Page not found");
		}
		console.error("Failed to load CMS page", err);
		error(500, "Unable to load page");
	}
};
