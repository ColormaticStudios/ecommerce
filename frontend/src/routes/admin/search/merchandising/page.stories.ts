import type { Meta, StoryObj } from "@storybook/sveltekit";
import { expect, userEvent, within } from "storybook/test";
import type { ComponentProps } from "svelte";
import type { components } from "$lib/api/generated/openapi";
import type {
	MerchandisingRule,
	MerchandisingPreview,
} from "$lib/api/domains/search-merchandising";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { makeCategory } from "$lib/storybook/factories";
import { renderRouteStory } from "$lib/storybook/render";
import { createApiStub } from "$lib/storybook/api";
import Page from "./+page.svelte";

type Data = ComponentProps<typeof Page>["data"];
const now = "2026-10-05T12:00:00Z";
const pin: MerchandisingRule = {
	id: 1,
	name: "Autumn jackets",
	rule_type: "pin",
	priority: 20,
	is_active: true,
	predicate: { query: { mode: "contains", value: "jacket" }, channel: "storefront" },
	action: { targets: [{ product_id: 101, product_name: "Field Jacket", position: 1 }] },
	starts_at: null,
	ends_at: "2026-11-01T00:00:00Z",
	version: 2,
	updated_by: 1,
	created_at: now,
	updated_at: now,
};
const secondary: MerchandisingRule = {
	...pin,
	id: 2,
	name: "Rainwear placement",
	priority: 10,
	version: 1,
	action: { targets: [{ product_id: 102, product_name: "Storm Shell", position: 1 }] },
};
const product = (id: number, name: string): components["schemas"]["Product"] => ({
	id,
	name,
	sku: name.toLowerCase().replaceAll(" ", "-"),
	description: "An everyday outer layer.",
	price: 129,
	stock: 10,
	images: [],
	price_range: { min: 129, max: 129 },
	options: [],
	variants: [],
	attributes: [],
	related_products: [],
	categories: [],
	seo: {
		title: null,
		description: null,
		canonical_path: null,
		og_image_media_id: null,
		noindex: false,
	},
	created_at: now,
	updated_at: now,
});
const field = product(101, "Field Jacket");
const shell = product(102, "Storm Shell");
const response: components["schemas"]["AdminProductSearchResponse"] = {
	items: [shell, field],
	facets: [],
	explanations: [
		{ product_id: 102, score: 15, adjusted_score: 15, components: [] },
		{ product_id: 101, score: 10, adjusted_score: 10, components: [] },
	],
	rule_decisions: [],
	metadata: {
		degraded: false,
		ranking_profile: "default",
		ranking_profile_version: 1,
		normalized_query: "jacket",
		applied_rewrites: [],
		did_you_mean: null,
		relaxed: false,
		indexed_at: now,
	},
	pagination: { page: 1, limit: 20, total: 2, total_pages: 1 },
};
const preview: MerchandisingPreview = {
	baseline: response,
	proposed: {
		...response,
		items: [field, shell],
		rule_decisions: [
			{
				rule_id: 1,
				rule_name: pin.name,
				rule_type: "pin",
				product_id: 101,
				product_name: field.name,
				outcome: "applied",
				reason: "pinned",
				position: 1,
			},
			{
				rule_id: 2,
				rule_name: secondary.name,
				rule_type: "pin",
				product_id: 102,
				product_name: shell.name,
				outcome: "conflict",
				reason: "position_occupied",
				position: 1,
			},
		],
	},
};
function data(overrides: Partial<Data> = {}): Data {
	return {
		...makeAdminLayoutData(),
		rules: [pin, secondary],
		categories: [makeCategory()],
		audit: [
			{
				id: 1,
				rule_id: 1,
				operation: "update",
				actor_id: 1,
				before: { ...pin, version: 1, priority: 10 },
				after: pin,
				created_at: now,
			},
		],
		errorMessages: [],
		...overrides,
	};
}
function api(state: Data) {
	return createApiStub({
		listAdminSearchMerchandisingRules: async () => state.rules,
		listAdminSearchMerchandisingAudit: async () => state.audit,
		listAllAdminSearchMerchandisingAudit: async () => state.audit,
		createAdminSearchMerchandisingRule: async () => pin,
		updateAdminSearchMerchandisingRule: async (_id, patch) => ({
			...pin,
			...patch,
			action: patch.action
				? {
						...patch.action,
						targets: patch.action.targets.map((target) => ({
							...target,
							product_name: target.product_id === 101 ? "Field Jacket" : "Storm Shell",
						})),
					}
				: pin.action,
			starts_at: patch.starts_at ?? null,
			ends_at: patch.ends_at ?? null,
		}),
		deleteAdminSearchMerchandisingRule: async () => undefined,
		previewAdminSearch: async () => preview,
	});
}
const meta = {
	title: "Routes/Admin/SearchMerchandising",
	component: RouteStoryHarness,
} satisfies Meta;
export default meta;
type Story = StoryObj;
export const Rules: Story = {
	render: () => {
		const state = data();
		return renderRouteStory({ component: Page, componentProps: { data: state }, api: api(state) });
	},
};
export const Empty: Story = {
	render: () => {
		const state = data({ rules: [], audit: [] });
		return renderRouteStory({ component: Page, componentProps: { data: state }, api: api(state) });
	},
};
export const EditingAndConflicts: Story = {
	render: () => {
		const state = data();
		return renderRouteStory({
			component: Page,
			componentProps: { data: { ...state, initialRule: pin, initialPreview: preview } },
			api: api(state),
		});
	},
};
export const ScheduledInactive: Story = {
	render: () => {
		const rule = {
			...pin,
			is_active: false,
			starts_at: "2026-11-01T00:00:00Z",
			ends_at: "2026-12-01T00:00:00Z",
		};
		const state = data({ rules: [rule] });
		return renderRouteStory({
			component: Page,
			componentProps: { data: { ...state, initialRule: rule } },
			api: api(state),
		});
	},
};
export const LoadError: Story = {
	render: () => {
		const state = data({
			rules: [],
			audit: [],
			errorMessages: ["Unable to load merchandising rules."],
		});
		return renderRouteStory({ component: Page, componentProps: { data: state }, api: api(state) });
	},
};

export const DeletedRuleHistory: Story = {
	render: () => {
		const state = data({
			rules: [],
			audit: [
				{
					id: 3,
					rule_id: pin.id,
					operation: "delete",
					actor_id: 1,
					before: pin,
					after: null,
					created_at: now,
				},
				{
					id: 2,
					rule_id: pin.id,
					operation: "create",
					actor_id: 1,
					before: null,
					after: pin,
					created_at: now,
				},
			],
		});
		return renderRouteStory({ component: Page, componentProps: { data: state }, api: api(state) });
	},
};

export const LongRuleNames: Story = {
	render: () => {
		const rule = {
			...pin,
			name: "Seasonal merchandising for waterproof jackets and everyday outdoor essentials",
			action: {
				targets: [
					{
						product_id: 101,
						product_name: "Waterproof Field Jacket with removable insulated lining",
						position: 1,
					},
				],
			},
		};
		const state = data({ rules: [rule, secondary] });
		return renderRouteStory({
			component: Page,
			componentProps: { data: { ...state, initialRule: rule, initialPreview: preview } },
			api: api(state),
		});
	},
};

export const ProductSelection: Story = {
	render: () => {
		const state = data({ rules: [], audit: [] });
		return renderRouteStory({ component: Page, componentProps: { data: state }, api: api(state) });
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.type(canvas.getByPlaceholderText("Search products"), "jacket");
		await userEvent.click(canvas.getByRole("button", { name: "Search" }));
		const result = await canvas.findByRole("button", { name: /Field Jacket/ });
		await userEvent.click(result);
		await expect(result).toHaveAttribute("aria-pressed", "true");
		await expect(within(result).getByText("Selected")).toBeVisible();
		await userEvent.click(result);
		await expect(result).toHaveAttribute("aria-pressed", "false");
		await userEvent.click(result);
		await expect(result).toHaveAttribute("aria-pressed", "true");
	},
};
