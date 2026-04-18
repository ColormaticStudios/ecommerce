import { expect, test } from "vitest";
import { makeShipment } from "$lib/storybook/factories";
import { shouldShowShipmentEmptyState } from "./page-state";

test("shows the shipment empty state when there are no shipments and no tracking error", () => {
	expect(shouldShowShipmentEmptyState([], "")).toBe(true);
});

test("hides the shipment empty state when tracking is unavailable", () => {
	expect(shouldShowShipmentEmptyState([], "Unable to load shipment tracking.")).toBe(false);
});

test("hides the shipment empty state when shipment data is present", () => {
	expect(shouldShowShipmentEmptyState([makeShipment()], "")).toBe(false);
});
