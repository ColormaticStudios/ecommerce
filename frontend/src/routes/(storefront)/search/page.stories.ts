import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeAttributeDefinition, makeBrand, makeProduct } from "$lib/storybook/factories";
import { makeRouteLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import SearchPage from "./+page.svelte";

type SearchPageData = ComponentProps<typeof SearchPage>["data"];

const meta = {
	title: "Routes/Search",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

const browseResults = [
	makeProduct({
		id: 101,
		sku: "field-jacket",
		name: "Field Jacket",
	}),
	makeProduct({
		id: 102,
		sku: "storm-shell",
		name: "Storm Shell",
		price: 164,
		stock: 5,
	}),
	makeProduct({
		id: 103,
		sku: "canvas-tote",
		name: "Canvas Tote",
		price: 58,
		stock: 3,
		images: [],
		cover_image: undefined,
	}),
];

function createData(overrides: Partial<SearchPageData> = {}): SearchPageData {
	return {
		...makeRouteLayoutData(),
		brandSlugs: [],
		categorySlugs: [],
		stockSelections: [],
		priceRanges: [],
		attributeSelections: {},
		facets: [],
		categories: [],
		metadata: null,
		suggestions: { suggestions: [], corrections: [], popular: [], trending: [] },
		results: [],
		brands: [makeBrand(), makeBrand({ id: 2, name: "Northline", slug: "northline" })],
		attributes: [
			makeAttributeDefinition(),
			makeAttributeDefinition({ id: 2, key: "waterproof", slug: "waterproof", type: "boolean" }),
		],
		errorMessage: "",
		searchQuery: "",
		draftQuery: "",
		brandSlug: "",
		hasVariantStock: false,
		attributeFilters: {},
		currentPage: 1,
		pageSize: 12,
		totalPages: 1,
		totalResults: 0,
		sortBy: "created_at",
		sortOrder: "desc",
		sortExplicit: false,
		rankingProfile: "",
		...overrides,
	};
}

export const BrowseAll: Story = {
	render: () =>
		renderRouteStory({
			component: SearchPage,
			componentProps: {
				data: createData({
					results: browseResults,
					totalResults: browseResults.length,
				}),
			},
		}),
};

export const Results: Story = {
	render: () =>
		renderRouteStory({
			component: SearchPage,
			componentProps: {
				data: createData({
					searchQuery: "jacket",
					sortBy: "relevance",
					draftQuery: "jacket",
					results: [
						makeProduct({ id: 101, sku: "field-jacket", name: "Field Jacket" }),
						makeProduct({ id: 102, sku: "storm-shell", name: "Storm Shell", price: 164, stock: 5 }),
					],
					totalResults: 2,
				}),
			},
		}),
};

export const NoMatches: Story = {
	render: () =>
		renderRouteStory({
			component: SearchPage,
			componentProps: {
				data: createData({
					searchQuery: "unobtainium",
					sortBy: "relevance",
					draftQuery: "unobtainium",
					brandSlug: "colormatic",
					totalResults: 0,
				}),
			},
		}),
};

export const LoadError: Story = {
	render: () =>
		renderRouteStory({
			component: SearchPage,
			componentProps: {
				data: createData({
					errorMessage: "Unable to load search results.",
				}),
			},
		}),
};

export const RankedNewArrivals: Story = {
	render: () =>
		renderRouteStory({
			component: SearchPage,
			componentProps: {
				data: createData({
					searchQuery: "jacket",
					draftQuery: "jacket",
					sortBy: "relevance",
					sortExplicit: true,
					rankingProfile: "new_arrivals",
					results: browseResults.slice(0, 2),
					totalResults: 2,
				}),
			},
		}),
};

const searchMetadata = {
	degraded: false,
	ranking_profile: "default",
	ranking_profile_version: 1,
	normalized_query: "jacket",
	applied_rewrites: [],
	did_you_mean: null,
	relaxed: false,
	indexed_at: null,
};
export const SpellingRecovery: Story = {
	render: () =>
		renderRouteStory({
			component: SearchPage,
			componentProps: {
				data: createData({
					searchQuery: "jakcet",
					draftQuery: "jakcet",
					metadata: { ...searchMetadata, did_you_mean: "jacket", relaxed: true },
					results: browseResults,
					totalResults: 3,
				}),
			},
		}),
};
export const PopularDiscovery: Story = {
	render: () =>
		renderRouteStory({
			component: SearchPage,
			componentProps: {
				data: createData({
					suggestions: {
						suggestions: [],
						corrections: [],
						popular: ["jacket", "tote"],
						trending: ["storm shell"],
					},
					results: browseResults,
					totalResults: 3,
				}),
			},
		}),
};
export const CatalogFallback: Story = {
	render: () =>
		renderRouteStory({
			component: SearchPage,
			componentProps: {
				data: createData({
					searchQuery: "jacket",
					draftQuery: "jacket",
					metadata: { ...searchMetadata, degraded: true, fallback_reason: "circuit_open" },
					results: browseResults,
					totalResults: 3,
				}),
			},
		}),
};

export const FacetSelections: Story = {
	render: () =>
		renderRouteStory({
			component: SearchPage,
			componentProps: {
				data: createData({
					searchQuery: "jacket",
					draftQuery: "jacket",
					brandSlugs: ["colormatic", "northline"],
					attributeSelections: { color: ["red", "blue"] },
					facets: [
						{
							name: "brand",
							label: "Brands",
							type: "terms",
							values: [
								{
									value: "colormatic",
									label: "Colormatic",
									count: 8,
									selected: true,
									disabled: false,
								},
								{
									value: "northline",
									label: "Northline",
									count: 4,
									selected: true,
									disabled: false,
								},
								{ value: "other", label: "Other", count: 0, selected: false, disabled: true },
							],
						},
						{
							name: "attribute:color",
							label: "Color",
							type: "attribute",
							values: [
								{ value: "red", label: "Red", count: 3, selected: true, disabled: false },
								{ value: "blue", label: "Blue", count: 0, selected: true, disabled: true },
							],
						},
					],
					results: browseResults,
					totalResults: 3,
				}),
			},
		}),
};
