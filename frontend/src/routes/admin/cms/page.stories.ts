import type { Meta, StoryObj } from "@storybook/sveltekit";
import { expect, userEvent, within } from "storybook/test";
import type { components } from "$lib/api/generated/openapi";
import type { API } from "$lib/api";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub, pendingPromise } from "$lib/storybook/api";
import { renderRouteStory } from "$lib/storybook/render";
import AdminCmsPage from "./+page.svelte";

type CmsPageResponse = components["schemas"]["CmsPageResponse"];
type CmsNavigationResponse = components["schemas"]["CmsNavigationResponse"];
type CmsGlobalRegionResponse = components["schemas"]["CmsGlobalRegionResponse"];
type CmsEntry = components["schemas"]["CmsEntry"];
type CmsEntryVersion = components["schemas"]["CmsEntryVersion"];
type CmsPageDeliveryResponse = components["schemas"]["CmsPageDeliveryResponse"];
type Pagination = components["schemas"]["Pagination"];
type CmsPageVariant = components["schemas"]["CmsPageVariant"];
type CmsAuditEvent = components["schemas"]["CmsAuditEvent"];

const timestamp = "2026-04-07T11:30:00.000Z";
const pagination: Pagination = { page: 1, limit: 50, total: 1, total_pages: 1 };

const meta = {
	title: "Routes/Admin/CMS",
	component: RouteStoryHarness,
	parameters: {
		backgrounds: { disable: true },
	},
} satisfies Meta;

export default meta;
type Story = StoryObj;

function makeEntry(overrides: Partial<CmsEntry>): CmsEntry {
	return {
		id: 1,
		entry_type: "page",
		key: "page:/shipping",
		status: "DRAFT",
		current_version_id: 11,
		published_version_id: 10,
		created_at: timestamp,
		updated_at: timestamp,
		...overrides,
	};
}

function makeVersion(overrides: Partial<CmsEntryVersion> = {}): CmsEntryVersion {
	return {
		id: 11,
		entry_id: 1,
		version_number: 2,
		schema_version: 1,
		payload: {
			blocks: [
				{ type: "hero", title: "Shipping", subtitle: "Fulfillment copy managed by CMS." },
				{ type: "rich_text", body: "Orders leave the warehouse Monday through Friday." },
			] as unknown as CmsEntryVersion["payload"]["blocks"],
		},
		created_by: null,
		change_summary: "Storybook draft content",
		created_at: timestamp,
		...overrides,
	};
}

const pageResponse: CmsPageResponse = {
	page: {
		id: 101,
		entry_id: 1,
		path: "/shipping",
		slug: "shipping",
		title: "Shipping",
		template_key: "default",
		visibility: "public",
		seo_metadata_id: null,
		is_homepage: false,
		created_at: timestamp,
		updated_at: timestamp,
	},
	entry: makeEntry({ id: 1, entry_type: "page", key: "page:/shipping" }),
	current_version: makeVersion(),
	published_version: makeVersion({ id: 10, version_number: 1 }),
	has_unpublished_draft: true,
};

const cleanPublishedPageResponse: CmsPageResponse = {
	...pageResponse,
	entry: makeEntry({
		id: 1,
		entry_type: "page",
		key: "page:/shipping",
		status: "PUBLISHED",
		current_version_id: 10,
		published_version_id: 10,
	}),
	current_version: makeVersion({ id: 10, version_number: 1 }),
	published_version: makeVersion({ id: 10, version_number: 1 }),
	has_unpublished_draft: false,
};

const navigationResponse: CmsNavigationResponse = {
	menu: {
		id: 201,
		entry_id: 2,
		key: "main",
		title: "Main navigation",
		location: "header",
		created_at: timestamp,
		updated_at: timestamp,
	},
	entry: makeEntry({
		id: 2,
		entry_type: "navigation",
		key: "navigation:main",
		status: "PUBLISHED",
		current_version_id: 20,
		published_version_id: 20,
	}),
	items: [
		{
			id: 301,
			menu_id: 201,
			parent_id: null,
			label: "Search",
			item_type: "internal",
			target_ref: "/search",
			url: "/search",
			sort_order: 1,
			is_enabled: true,
		},
	],
	current_version: makeVersion({ id: 20, entry_id: 2, version_number: 1 }),
	published_version: makeVersion({ id: 20, entry_id: 2, version_number: 1 }),
	has_unpublished_draft: false,
};

const globalRegionResponse: CmsGlobalRegionResponse = {
	region: {
		id: 401,
		entry_id: 3,
		key: "announcement",
		title: "Announcement",
		region: "announcement_bar",
		created_at: timestamp,
		updated_at: timestamp,
	},
	entry: makeEntry({
		id: 3,
		entry_type: "global",
		key: "global:announcement",
		status: "DRAFT",
		current_version_id: 31,
		published_version_id: 30,
	}),
	current_version: makeVersion({
		id: 31,
		entry_id: 3,
		payload: {
			blocks: [
				{
					type: "promo_banner",
					title: "Free domestic shipping over $100",
					body: "Applied automatically at checkout.",
					link: { label: "Shop now", url: "/search" },
				},
			] as unknown as CmsEntryVersion["payload"]["blocks"],
		},
	}),
	published_version: makeVersion({ id: 30, entry_id: 3, version_number: 1 }),
	has_unpublished_draft: true,
};

const footerRegionResponse: CmsGlobalRegionResponse = {
	...globalRegionResponse,
	region: {
		...globalRegionResponse.region,
		id: 402,
		entry_id: 4,
		key: "site-footer",
		title: "Site footer",
		region: "footer",
	},
	entry: makeEntry({
		id: 4,
		entry_type: "global",
		key: "global:footer",
		status: "PUBLISHED",
		current_version_id: 41,
		published_version_id: 40,
	}),
	current_version: makeVersion({
		id: 41,
		entry_id: 4,
		payload: {
			blocks: [
				{
					type: "footer",
					brand_name: "Colormatic Supply",
					tagline: "Useful goods, clearly presented.",
					columns: [
						{ title: "Shop", links: [{ label: "New arrivals", url: "/search" }] },
						{ title: "Help", links: [{ label: "Shipping", url: "/shipping" }] },
					],
					social_links: [{ label: "Instagram", url: "https://instagram.com" }],
					copyright: "© 2026 Colormatic Supply",
					layout: "columns",
				},
			] as unknown as CmsEntryVersion["payload"]["blocks"],
		},
	}),
	published_version: makeVersion({ id: 40, entry_id: 4, version_number: 1 }),
	has_unpublished_draft: true,
};

const deliveryResponse: CmsPageDeliveryResponse = {
	targeting_rules: [],
	recent_publications: [],
};

function createCmsApi(
	variants: CmsPageVariant[] = [],
	auditEvents: CmsAuditEvent[] = [],
	pageResponses: CmsPageResponse[] = [pageResponse],
	overrides: Partial<API> = {}
) {
	return createApiStub({
		listAdminCmsPages: async () => ({ data: pageResponses, pagination }),
		listAdminCmsNavigation: async () => ({ data: [navigationResponse], pagination }),
		listAdminCmsGlobalRegions: async () => ({ data: [globalRegionResponse], pagination }),
		getAdminCmsLocales: async () => ({
			locales: [
				{
					code: "en-US",
					name: "English (United States)",
					enabled: true,
					is_default: true,
					fallback_locale: null,
				},
				{
					code: "fr-CA",
					name: "French (Canada)",
					enabled: true,
					is_default: false,
					fallback_locale: "en-US",
				},
			],
		}),
		updateAdminCmsLocales: async (input) => input,
		listAdminCmsPageVariants: async () => variants,
		listAdminCmsAuditEvents: async () => auditEvents,
		exportAdminCmsContent: async () => ({
			schema_version: 1,
			exported_at: new Date().toISOString(),
			locales: [],
			pages: [],
			navigation: [],
			global_regions: [],
			variants: [],
		}),
		restoreAdminCmsContent: async () => undefined,
		previewAdminCmsRestore: async (content) => ({
			valid: true,
			schema_version: content.schema_version,
			pages: content.pages.length,
			navigation: content.navigation.length,
			global_regions: content.global_regions.length,
			variants: content.variants.length,
			warnings: [],
			errors: [],
		}),
		getAdminCmsGovernance: async () => ({
			approval_required: true,
			invalidation_webhook_url: "",
			roles: [{ subject: "admin@example.com", role: "publisher" }],
		}),
		updateAdminCmsGovernance: async (input) => input,
		getAdminCmsOperations: async () => ({
			pending_schedules: 1,
			active_experiments: 1,
			invalidations: [
				{
					id: 7,
					entry_id: pageResponse.entry.id,
					variant_id: null,
					reason: "page.published",
					status: "failed",
					attempts: 3,
					last_error: "Webhook timed out",
					created_at: new Date().toISOString(),
					sent_at: null,
				},
			],
		}),
		retryAdminCmsInvalidation: async () => undefined,
		createAdminCmsPage: async () => pageResponse,
		updateAdminCmsPage: async () => pageResponse,
		publishAdminCmsPage: async () => ({ ...pageResponse, has_unpublished_draft: false }),
		unpublishAdminCmsPage: async () => ({
			...pageResponse,
			entry: { ...pageResponse.entry, published_version_id: null },
			has_unpublished_draft: true,
		}),
		discardAdminCmsPageDraft: async () => cleanPublishedPageResponse,
		deleteAdminCmsPage: async () => undefined,
		previewAdminCmsPayload: async () => ({
			blocks: [
				{ key: "hero:0", type: "hero", status: "static", item_count: 0, messages: [] },
				{ key: "rich_text:1", type: "rich_text", status: "static", item_count: 0, messages: [] },
			],
		}),
		getAdminCmsPageDelivery: async () => deliveryResponse,
		updateAdminCmsPageDelivery: async () => deliveryResponse,
		getAdminCmsPageSEO: async () => ({
			metadata: {
				title: "Shipping",
				description: "Shipping information",
				canonical_url: "/shipping",
				robots: "index_follow",
				og_title: "Shipping",
				og_description: "Shipping information",
				og_image_media_id: null,
				twitter_card: "summary",
				twitter_title: "Shipping",
				twitter_description: "Shipping information",
				twitter_image_media_id: null,
				json_ld: [],
			},
			issues: [],
		}),
		listAdminCmsRedirects: async () => [],
		deleteAdminCmsRedirect: async () => undefined,
		createAdminCmsNavigation: async () => navigationResponse,
		updateAdminCmsNavigation: async () => navigationResponse,
		publishAdminCmsNavigation: async () => navigationResponse,
		unpublishAdminCmsNavigation: async () => ({
			...navigationResponse,
			entry: { ...navigationResponse.entry, published_version_id: null },
			has_unpublished_draft: true,
		}),
		discardAdminCmsNavigationDraft: async () => navigationResponse,
		deleteAdminCmsNavigation: async () => undefined,
		createAdminCmsGlobalRegion: async () => globalRegionResponse,
		updateAdminCmsGlobalRegion: async () => globalRegionResponse,
		publishAdminCmsGlobalRegion: async () => ({
			...globalRegionResponse,
			has_unpublished_draft: false,
		}),
		unpublishAdminCmsGlobalRegion: async () => ({
			...globalRegionResponse,
			entry: { ...globalRegionResponse.entry, published_version_id: null },
			has_unpublished_draft: true,
		}),
		discardAdminCmsGlobalRegionDraft: async () => globalRegionResponse,
		deleteAdminCmsGlobalRegion: async () => undefined,
		getAdminPreviewSession: async () => ({ active: false, expires_at: null }),
		startAdminPreview: async () => ({ active: true, expires_at: null }),
		stopAdminPreview: async () => ({ active: false, expires_at: null }),
		...overrides,
	});
}

export const Loading: Story = {
	render: () =>
		renderRouteStory({
			component: AdminCmsPage,
			api: createCmsApi([], [], [], {
				listAdminCmsPages: async () => pendingPromise(),
				listAdminCmsNavigation: async () => pendingPromise(),
				listAdminCmsGlobalRegions: async () => pendingPromise(),
				getAdminCmsLocales: async () => pendingPromise(),
				listAdminCmsRedirects: async () => pendingPromise(),
				getAdminCmsGovernance: async () => pendingPromise(),
				getAdminCmsOperations: async () => pendingPromise(),
			}),
		}),
};

export const Loaded: Story = {
	render: () =>
		renderRouteStory({
			component: AdminCmsPage,
			api: createCmsApi(),
		}),
};

export const CleanPublishedPage: Story = {
	render: () =>
		renderRouteStory({
			component: AdminCmsPage,
			api: createCmsApi([], [], [cleanPublishedPageResponse]),
		}),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(await canvas.findByRole("button", { name: /shipping/i }));
		await expect((await canvas.findAllByText("Published"))[0]).toBeVisible();
		await expect(canvas.getByRole("button", { name: "Publish" })).toBeDisabled();
	},
};

export const DeletePageConfirmation: Story = {
	render: () =>
		renderRouteStory({
			component: AdminCmsPage,
			api: createCmsApi(),
		}),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(await canvas.findByRole("button", { name: /shipping/i }));
		await userEvent.click(await canvas.findByRole("button", { name: "Delete" }));
		await expect(await canvas.findByRole("alertdialog")).toBeVisible();
		await expect(await canvas.findByText('Delete "Shipping"?')).toBeVisible();
	},
};

export const LocalizationGovernance: Story = {
	render: () =>
		renderRouteStory({
			component: AdminCmsPage,
			api: createCmsApi(
				[
					{
						id: 21,
						page_id: pageResponse.page.id,
						entry_id: pageResponse.entry.id,
						locale: "fr-CA",
						market: "CA",
						path: "/fr/livraison",
						slug: "livraison",
						title: "Livraison",
						payload: pageResponse.current_version?.payload ?? {},
						status: "published",
						revision: 3,
						submitted_by: "editor@example.com",
						approved_by: "publisher@example.com",
						published_at: timestamp,
						created_at: timestamp,
						updated_at: timestamp,
					},
				],
				[
					{
						id: 9,
						entry_id: pageResponse.entry.id,
						version_id: null,
						variant_id: 21,
						action: "variant.published",
						actor: "publisher@example.com",
						detail: "French Canadian shipping copy approved",
						created_at: timestamp,
					},
				]
			),
		}),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(await canvas.findByRole("button", { name: /shipping/i }));
		await userEvent.click(await canvas.findByRole("button", { name: /fr-ca \/ ca/i }));
	},
};

export const Operations: Story = {
	render: () =>
		renderRouteStory({
			component: AdminCmsPage,
			api: createCmsApi(),
		}),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(await canvas.findByRole("button", { name: /operations/i }));
		await expect(await canvas.findByText("Governance")).toBeVisible();
		await expect(await canvas.findByText("Invalidations")).toBeVisible();
	},
};

export const GlobalFooterBuilder: Story = {
	render: () =>
		renderRouteStory({
			component: AdminCmsPage,
			api: createCmsApi([], [], [pageResponse], {
				listAdminCmsGlobalRegions: async () => ({
					data: [globalRegionResponse, footerRegionResponse],
					pagination: { ...pagination, total: 2 },
				}),
			}),
		}),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(await canvas.findByRole("tab", { name: "Global" }));
		await userEvent.click(await canvas.findByRole("button", { name: /site footer/i }));
		await expect(await canvas.findByText("Brand and layout")).toBeVisible();
	},
};

export const BackupRestoreConfirmation: Story = {
	render: () =>
		renderRouteStory({
			component: AdminCmsPage,
			api: createCmsApi(),
		}),
	play: async ({ canvasElement }) => {
		const input = canvasElement.querySelector<HTMLInputElement>(
			'input[type="file"][accept*="json"]'
		);
		if (!input) throw new Error("CMS restore file input was not rendered");
		const backup = {
			schema_version: 1,
			exported_at: timestamp,
			locales: [],
			pages: [],
			navigation: [],
			global_regions: [],
			variants: [],
		};
		await userEvent.upload(
			input,
			new File([JSON.stringify(backup)], "cms-export-2026-06-21.json", {
				type: "application/json",
			})
		);
	},
};

export const VisualEditValidationError: Story = {
	render: () =>
		renderRouteStory({
			component: AdminCmsPage,
			api: createCmsApi([], [], [pageResponse], {
				updateAdminCmsPage: async () => {
					throw {
						body: { error: "invalid cms page: payload.blocks[1].body is required" },
					};
				},
			}),
		}),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(await canvas.findByRole("button", { name: /shipping/i }));
		await userEvent.click(await canvas.findByRole("button", { name: "Edit mode" }));
		await userEvent.click(await canvas.findByRole("button", { name: "Save" }));
		await expect(await canvas.findByText("Block #2 is missing body text.")).toBeVisible();
	},
};

export const Empty: Story = {
	render: () =>
		renderRouteStory({
			component: AdminCmsPage,
			api: createCmsApi([], [], [], {
				listAdminCmsPages: async () => ({ data: [], pagination: { ...pagination, total: 0 } }),
				listAdminCmsNavigation: async () => ({ data: [], pagination: { ...pagination, total: 0 } }),
				listAdminCmsGlobalRegions: async () => ({
					data: [],
					pagination: { ...pagination, total: 0 },
				}),
				listAdminCmsRedirects: async () => [],
			}),
		}),
};

export const LoadError: Story = {
	render: () =>
		renderRouteStory({
			component: AdminCmsPage,
			api: createCmsApi([], [], [], {
				listAdminCmsPages: async () => {
					throw new Error("CMS load failed");
				},
				listAdminCmsNavigation: async () => ({ data: [], pagination }),
				listAdminCmsGlobalRegions: async () => ({ data: [], pagination }),
			}),
		}),
};
