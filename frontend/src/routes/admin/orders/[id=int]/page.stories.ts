import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import type { components } from "$lib/api/generated/openapi";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { createApiStub } from "$lib/storybook/api";
import { makeOrder, makeOrderItem, makeProduct } from "$lib/storybook/factories";
import { makeAdminLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import AdminOrderDetailPage from "./+page.svelte";

type AdminOrderDetailData = ComponentProps<typeof AdminOrderDetailPage>["data"];
type OrderPaymentLedger = components["schemas"]["OrderPaymentLedger"];

const order = makeOrder({
	id: 501,
	status: "PAID",
	total: 258,
	items: [
		makeOrderItem({
			id: 401,
			quantity: 1,
			product: makeProduct({ id: 101, sku: "field-jacket", name: "Field Jacket" }),
		}),
		makeOrderItem({
			id: 402,
			quantity: 1,
			price: 129,
			product: makeProduct({
				id: 102,
				sku: "travel-tote",
				name: "Travel Tote",
				cover_image:
					"https://images.unsplash.com/photo-1542291026-7eec264c27ff?auto=format&fit=crop&w=900&q=80",
			}),
			variant_sku: "travel-tote-default",
			variant_title: "Canvas",
		}),
	],
});

const payments: OrderPaymentLedger = {
	order_id: order.id,
	intents: [
		{
			id: 701,
			order_id: order.id,
			snapshot_id: 601,
			provider: "stripe",
			status: "CAPTURED",
			authorized_amount: 258,
			captured_amount: 258,
			refundable_amount: 258,
			currency: "USD",
			version: 2,
			created_at: "2026-04-07T12:05:00.000Z",
			updated_at: "2026-04-07T12:06:00.000Z",
			transactions: [
				{
					id: 801,
					operation: "AUTHORIZE",
					provider_txn_id: "txn_auth_801",
					idempotency_key: "story-authorize",
					amount: 258,
					status: "SUCCEEDED",
					raw_response_redacted: "{}",
					created_at: "2026-04-07T12:05:00.000Z",
					updated_at: "2026-04-07T12:05:00.000Z",
				},
				{
					id: 802,
					operation: "CAPTURE",
					provider_txn_id: "txn_capture_802",
					idempotency_key: "story-capture",
					amount: 258,
					status: "SUCCEEDED",
					raw_response_redacted: "{}",
					created_at: "2026-04-07T12:06:00.000Z",
					updated_at: "2026-04-07T12:06:00.000Z",
				},
			],
		},
	],
};

const meta = {
	title: "Routes/Admin/Order Detail",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<AdminOrderDetailData> = {}): AdminOrderDetailData {
	return {
		...makeAdminLayoutData(),
		order,
		payments,
		errorMessage: "",
		paymentErrorMessage: "",
		...overrides,
	};
}

function createApi() {
	return createApiStub({
		updateOrderStatus: async (_id, payload) => ({ ...order, status: payload.status }),
		getAdminOrderPayments: async () => payments,
		captureAdminOrderPayment: async () => ({
			message: "Payment captured.",
			order: { ...order, status: "PAID" },
			payment_intent: payments.intents[0],
			transaction: payments.intents[0].transactions[1],
		}),
		voidAdminOrderPayment: async () => ({
			message: "Payment voided.",
			order: { ...order, status: "CANCELLED" },
			payment_intent: { ...payments.intents[0], status: "VOIDED" },
			transaction: payments.intents[0].transactions[0],
		}),
		refundAdminOrderPayment: async () => ({
			message: "Payment refunded.",
			order: { ...order, status: "REFUNDED" },
			payment_intent: { ...payments.intents[0], status: "REFUNDED", refundable_amount: 0 },
			transaction: payments.intents[0].transactions[0],
		}),
	});
}

export const Populated: Story = {
	render: () =>
		renderRouteStory({
			component: AdminOrderDetailPage,
			componentProps: { data: createData() },
			api: createApi(),
		}),
};

export const PaymentLoadError: Story = {
	render: () =>
		renderRouteStory({
			component: AdminOrderDetailPage,
			componentProps: {
				data: createData({
					payments: null,
					paymentErrorMessage: "Unable to load payment activity.",
				}),
			},
			api: createApi(),
		}),
};

export const NotFound: Story = {
	render: () =>
		renderRouteStory({
			component: AdminOrderDetailPage,
			componentProps: {
				data: createData({
					order: null,
					payments: null,
					errorMessage: "Order not found.",
				}),
			},
			api: createApi(),
		}),
};
