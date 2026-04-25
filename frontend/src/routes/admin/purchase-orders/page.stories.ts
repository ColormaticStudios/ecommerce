import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
import { makeProduct, makeVariant } from "$lib/storybook/factories";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import AdminPurchaseOrdersPage from "./+page.svelte";

type PurchaseOrdersData = ComponentProps<typeof AdminPurchaseOrdersPage>["data"];

const now = new Date("2026-04-24T19:00:00.000Z").toISOString();

const products = [
	makeProduct({
		id: 101,
		name: "Field Jacket",
		sku: "field-jacket",
		variants: [
			makeVariant({ id: 101, title: "Small", sku: "field-jacket-s" }),
			makeVariant({ id: 102, title: "Medium", sku: "field-jacket-m" }),
		],
		default_variant_id: 101,
		default_variant_sku: "field-jacket-s",
	}),
];

const data: PurchaseOrdersData = {
	...makeAdminLayoutData(),
	products,
	errorMessages: [],
	purchaseOrders: {
		items: [
			{
				id: 1,
				status: "ISSUED",
				supplier_id: null,
				supplier: {
					id: 1,
					name: "North Supply",
					email: "",
					notes: "",
					created_at: now,
					updated_at: now,
				},
				notes: "Spring restock",
				issued_at: now,
				received_at: null,
				cancelled_at: null,
				created_at: now,
				updated_at: now,
				items: [
					{
						id: 1,
						product_variant_id: 101,
						quantity_ordered: 12,
						quantity_received: 4,
						unit_cost: 60,
					},
				],
			},
		],
	},
};

const meta = {
	title: "Routes/Admin/Purchase Orders",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const Active: Story = {
	render: () =>
		renderRouteStory({
			component: AdminPurchaseOrdersPage,
			componentProps: { data },
			api: createApiStub({
				listAdminProducts: async () => ({
					data: products,
					pagination: { page: 1, limit: 20, total: products.length, total_pages: 1 },
				}),
				createAdminPurchaseOrder: async () => data.purchaseOrders.items[0],
				issueAdminPurchaseOrder: async () => data.purchaseOrders.items[0],
				cancelAdminPurchaseOrder: async () => ({
					...data.purchaseOrders.items[0],
					status: "CANCELLED",
				}),
				receiveAdminPurchaseOrder: async () => ({
					purchase_order: { ...data.purchaseOrders.items[0], status: "PARTIALLY_RECEIVED" },
					receipt: {
						id: 1,
						purchase_order_id: 1,
						received_at: now,
						notes: "",
						items: [
							{ id: 1, purchase_order_item_id: 1, product_variant_id: 101, quantity_received: 2 },
						],
					},
				}),
			}),
		}),
};
