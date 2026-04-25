import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
import { makeProduct, makeVariant } from "$lib/storybook/factories";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import AdminInventoryPage from "./+page.svelte";

type AdminInventoryData = ComponentProps<typeof AdminInventoryPage>["data"];

const now = new Date("2026-04-24T19:00:00.000Z");

const meta = {
	title: "Routes/Admin/Inventory",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<AdminInventoryData> = {}): AdminInventoryData {
	const products = [
		makeProduct({
			id: 101,
			sku: "field-jacket",
			name: "Field Jacket",
			stock: 8,
			variants: [
				makeVariant({ id: 101, sku: "field-jacket-s", title: "Small", stock: 3 }),
				makeVariant({ id: 102, sku: "field-jacket-m", title: "Medium", stock: 5 }),
			],
			default_variant_id: 101,
			default_variant_sku: "field-jacket-s",
		}),
		makeProduct({
			id: 202,
			sku: "canvas-tote",
			name: "Canvas Tote",
			stock: 0,
			variants: [makeVariant({ id: 202, sku: "canvas-tote-default", title: "Default", stock: 0 })],
			default_variant_id: 202,
			default_variant_sku: "canvas-tote-default",
		}),
	];
	return {
		...makeAdminLayoutData(),
		reservations: {
			items: [
				{
					id: 1,
					product_variant_id: 101,
					quantity: 2,
					status: "ACTIVE",
					expires_at: new Date(now.getTime() + 4 * 60 * 1000).toISOString(),
					owner_type: "ORDER",
					owner_id: null,
					checkout_session_id: 41,
					order_id: 501,
					created_at: new Date(now.getTime() - 2 * 60 * 1000).toISOString(),
					updated_at: new Date(now.getTime() - 2 * 60 * 1000).toISOString(),
				},
				{
					id: 2,
					product_variant_id: 202,
					quantity: 1,
					status: "CONSUMED",
					expires_at: new Date(now.getTime() + 10 * 60 * 1000).toISOString(),
					owner_type: "ORDER",
					owner_id: null,
					checkout_session_id: 42,
					order_id: 502,
					created_at: new Date(now.getTime() - 20 * 60 * 1000).toISOString(),
					updated_at: new Date(now.getTime() - 15 * 60 * 1000).toISOString(),
				},
			],
		},
		alerts: {
			items: [
				{
					id: 11,
					product_variant_id: 101,
					alert_type: "LOW_STOCK",
					status: "OPEN",
					available: 3,
					threshold: 5,
					opened_at: new Date(now.getTime() - 12 * 60 * 1000).toISOString(),
					acked_at: null,
					acked_by_type: undefined,
					acked_by_id: null,
					resolved_at: null,
					resolved_by_type: undefined,
					resolved_by_id: null,
					created_at: new Date(now.getTime() - 12 * 60 * 1000).toISOString(),
					updated_at: new Date(now.getTime() - 12 * 60 * 1000).toISOString(),
				},
				{
					id: 12,
					product_variant_id: 202,
					alert_type: "OUT_OF_STOCK",
					status: "ACKED",
					available: 0,
					threshold: 4,
					opened_at: new Date(now.getTime() - 30 * 60 * 1000).toISOString(),
					acked_at: new Date(now.getTime() - 20 * 60 * 1000).toISOString(),
					acked_by_type: "admin",
					acked_by_id: null,
					resolved_at: null,
					resolved_by_type: undefined,
					resolved_by_id: null,
					created_at: new Date(now.getTime() - 30 * 60 * 1000).toISOString(),
					updated_at: new Date(now.getTime() - 20 * 60 * 1000).toISOString(),
				},
			],
		},
		thresholds: {
			items: [
				{
					id: 1,
					product_variant_id: null,
					low_stock_quantity: 5,
					created_at: new Date(now.getTime() - 60 * 60 * 1000).toISOString(),
					updated_at: new Date(now.getTime() - 60 * 60 * 1000).toISOString(),
				},
				{
					id: 2,
					product_variant_id: 202,
					low_stock_quantity: 4,
					created_at: new Date(now.getTime() - 45 * 60 * 1000).toISOString(),
					updated_at: new Date(now.getTime() - 45 * 60 * 1000).toISOString(),
				},
			],
		},
		status: ["ACTIVE"],
		alertStatus: ["OPEN"],
		products,
		productTotal: products.length,
		limit: 100,
		errorMessages: [],
		...overrides,
	};
}

export const Active: Story = {
	render: () =>
		renderRouteStory({
			component: AdminInventoryPage,
			componentProps: { data: createData() },
			api: createApiStub({
				listAdminInventoryReservations: async () => createData().reservations,
				listAdminInventoryAlerts: async () => createData().alerts,
				listAdminInventoryThresholds: async () => createData().thresholds,
				ackAdminInventoryAlert: async () => createData().alerts.items[0],
				resolveAdminInventoryAlert: async () => ({
					...createData().alerts.items[0],
					status: "RESOLVED",
					resolved_at: now.toISOString(),
				}),
				upsertAdminInventoryThreshold: async () => createData().thresholds.items[0],
				deleteAdminInventoryThreshold: async () => ({ message: "Inventory threshold deleted" }),
				createAdminInventoryAdjustment: async () => ({
					adjustment: {
						id: 31,
						inventory_item_id: 101,
						product_variant_id: 101,
						quantity_delta: 1,
						reason_code: "CORRECTION",
						notes: "Story adjustment",
						actor_type: "admin",
						actor_id: null,
						approved_by_type: "",
						approved_by_id: null,
						approved_at: null,
						created_at: now.toISOString(),
						updated_at: now.toISOString(),
					},
					availability: {
						product_variant_id: 101,
						on_hand: 9,
						reserved: 0,
						available: 9,
					},
				}),
				runAdminInventoryReconciliation: async () => ({
					checked_at: now.toISOString(),
					issues: [],
				}),
				getAdminInventoryTimeline: async () => ({
					product_variant_id: 101,
					adjustments: [],
					movements: [],
					reservations: [],
				}),
				listAdminProducts: async () => ({
					data: createData().products,
					pagination: { page: 1, limit: 20, total: 2, total_pages: 1 },
				}),
			}),
		}),
};

export const Empty: Story = {
	render: () =>
		renderRouteStory({
			component: AdminInventoryPage,
			componentProps: {
				data: createData({ reservations: { items: [] }, alerts: { items: [] } }),
			},
			api: createApiStub({
				listAdminInventoryReservations: async () => ({ items: [] }),
				listAdminInventoryAlerts: async () => ({ items: [] }),
				listAdminInventoryThresholds: async () => createData().thresholds,
				ackAdminInventoryAlert: async () => createData().alerts.items[0],
				resolveAdminInventoryAlert: async () => createData().alerts.items[0],
				upsertAdminInventoryThreshold: async () => createData().thresholds.items[0],
				deleteAdminInventoryThreshold: async () => ({ message: "Inventory threshold deleted" }),
				createAdminInventoryAdjustment: async () => ({
					adjustment: {
						id: 31,
						inventory_item_id: 101,
						product_variant_id: 101,
						quantity_delta: 1,
						reason_code: "CORRECTION",
						notes: "",
						actor_type: "admin",
						actor_id: null,
						approved_by_type: "",
						approved_by_id: null,
						approved_at: null,
						created_at: now.toISOString(),
						updated_at: now.toISOString(),
					},
					availability: {
						product_variant_id: 101,
						on_hand: 1,
						reserved: 0,
						available: 1,
					},
				}),
				runAdminInventoryReconciliation: async () => ({
					checked_at: now.toISOString(),
					issues: [],
				}),
				getAdminInventoryTimeline: async () => ({
					product_variant_id: 101,
					adjustments: [],
					movements: [],
					reservations: [],
				}),
				listAdminProducts: async () => ({
					data: [],
					pagination: { page: 1, limit: 20, total: 0, total_pages: 1 },
				}),
			}),
		}),
};
