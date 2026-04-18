import type { ShipmentModel } from "$lib/models";

export function shouldShowShipmentEmptyState(
	shipments: ShipmentModel[],
	trackingErrorMessage: string
): boolean {
	return shipments.length === 0 && trackingErrorMessage.trim().length === 0;
}
