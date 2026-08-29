import type { OrderModel, ShipmentModel } from "$lib/models";
import type { LocalizationRuntime } from "$lib/localization/runtime";

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

export function localizeOrderStatus(
	localization: LocalizationRuntime,
	status: OrderModel["status"]
): string {
	switch (status) {
		case "PENDING":
			return localization.translate("storefront.order.status.pending", "Pending");
		case "PAID":
			return localization.translate("storefront.order.status.paid", "Paid");
		case "FAILED":
			return localization.translate("storefront.order.status.failed", "Failed");
		case "SHIPPED":
			return localization.translate("storefront.order.status.shipped", "Shipped");
		case "DELIVERED":
			return localization.translate("storefront.order.status.delivered", "Delivered");
		case "CANCELLED":
			return localization.translate("storefront.order.status.cancelled", "Cancelled");
		case "REFUNDED":
			return localization.translate("storefront.order.status.refunded", "Refunded");
	}
}

export function localizeShipmentStatus(
	localization: LocalizationRuntime,
	status: ShipmentModel["status"] | ShipmentModel["tracking_events"][number]["status"]
): string {
	switch (status) {
		case "QUOTED":
			return localization.translate("storefront.shipment.status.quoted", "Quoted");
		case "LABEL_PURCHASED":
			return localization.translate(
				"storefront.shipment.status.label_purchased",
				"Label purchased"
			);
		case "IN_TRANSIT":
			return localization.translate("storefront.shipment.status.in_transit", "In transit");
		case "DELIVERED":
			return localization.translate("storefront.shipment.status.delivered", "Delivered");
		case "EXCEPTION":
			return localization.translate("storefront.shipment.status.exception", "Exception");
	}
}
