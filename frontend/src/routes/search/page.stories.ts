import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import {
	makeAttributeDefinition,
	makeBrand,
	makeProduct,
	makeStorefrontSettings,
} from "$lib/storybook/factories";
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

function createData(overrides: Partial<SearchPageData> = {}): SearchPageData {
	return {
		...makeRouteLayoutData(),
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
		storefront: makeStorefrontSettings(),
		...overrides,
	};
}

export const BrowseAll: Story = {
	render: () =>
		renderRouteStory({
			component: SearchPage,
			componentProps: { data: createData() },
		}),
};

export const Results: Story = {
	render: () =>
		renderRouteStory({
			component: SearchPage,
			componentProps: {
				data: createData({
					searchQuery: "jacket",
					draftQuery: "jacket",
					results: [
						makeProduct({ id: 101, name: "Field Jacket" }),
						makeProduct({ id: 102, name: "Storm Shell", price: 164, stock: 5 }),
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
