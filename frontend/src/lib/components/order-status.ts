import type { OrderModel } from "$lib/models";

export type OrderStatusTone = "neutral" | "info" | "success" | "warning" | "danger";

export function getOrderStatusTone(status: OrderModel["status"]): OrderStatusTone {
	switch (status) {
		case "PAID":
		case "DELIVERED":
			return "success";
		case "SHIPPED":
			return "info";
		case "FAILED":
			return "danger";
		case "PENDING":
			return "warning";
		default:
			return "neutral";
	}
}

export function formatOrderStatusLabel(status: OrderModel["status"]): string {
	return status
		.toLowerCase()
		.split("_")
		.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
		.join(" ");
}
