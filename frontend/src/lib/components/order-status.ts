import type { OrderModel } from "$lib/models";

export type OrderStatusTone = "neutral" | "info" | "success" | "warning" | "danger";

const orderStatusTones = {
	PENDING: "warning",
	PAID: "success",
	FAILED: "danger",
	SHIPPED: "info",
	DELIVERED: "success",
	CANCELLED: "neutral",
	REFUNDED: "info",
} satisfies Record<OrderModel["status"], OrderStatusTone>;

export function getOrderStatusTone(status: OrderModel["status"]): OrderStatusTone {
	return orderStatusTones[status];
}

export function formatOrderStatusLabel(status: OrderModel["status"]): string {
	return status
		.toLowerCase()
		.split("_")
		.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
		.join(" ");
}
