import type { Meta, StoryObj } from "@storybook/sveltekit";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeRouteLayoutData } from "$lib/storybook/layout";
import { createApiStub } from "$lib/storybook/api";
import { renderRouteStory } from "$lib/storybook/render";
import StorefrontLayout from "./+layout.svelte";

const meta = {
	title: "Routes/Storefront Layout",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const ManagedCmsChrome: Story = {
	render: () =>
		renderRouteStory({
			component: StorefrontLayout,
			api: createApiStub({ viewCartSummary: async () => 0 }),
			componentProps: {
				data: makeRouteLayoutData({
					cmsNavigation: {
						id: 1,
						key: "main",
						title: "Main",
						location: "header",
						hasUnpublishedDraft: false,
						items: [
							{
								id: 1,
								parentId: null,
								label: "Shipping",
								itemType: "page",
								targetRef: "/shipping",
								url: "/shipping",
								sortOrder: 1,
								isEnabled: true,
								children: [],
							},
							{
								id: 2,
								parentId: null,
								label: "Returns",
								itemType: "page",
								targetRef: "/returns",
								url: "/returns",
								sortOrder: 2,
								isEnabled: true,
								children: [],
							},
						],
					},
					cmsGlobalRegions: {
						announcement_bar: {
							id: 1,
							key: "announcement",
							title: "Announcement",
							region: "announcement_bar",
							hasUnpublishedDraft: false,
							blocks: [
								{
									type: "promo_banner",
									title: "Free domestic shipping over $100",
									body: "Applied automatically at checkout.",
									link: { label: "Shop now", url: "/search" },
								},
							],
						},
						trust_strip: {
							id: 2,
							key: "trust",
							title: "Trust strip",
							region: "trust_strip",
							hasUnpublishedDraft: false,
							blocks: [
								{ type: "rich_text", body: "Secure checkout" },
								{ type: "rich_text", body: "Fast dispatch" },
								{ type: "rich_text", body: "Repairable goods" },
							],
						},
						footer: {
							id: 3,
							key: "site-footer",
							title: "Site footer",
							region: "footer",
							hasUnpublishedDraft: false,
							blocks: [
								{
									type: "footer",
									brand_name: "Colormatic Supply",
									tagline: "Useful goods, clearly presented.",
									columns: [
										{
											title: "Shop",
											links: [{ label: "New arrivals", url: "/search" }],
										},
										{
											title: "Help",
											links: [{ label: "Shipping", url: "/shipping" }],
										},
									],
									social_links: [{ label: "Instagram", url: "https://instagram.com" }],
									copyright: "© 2026 Colormatic Supply",
									layout: "columns",
								},
							],
						},
					},
				}),
			},
		}),
};
