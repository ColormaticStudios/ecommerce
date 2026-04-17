import { expect, test } from "vitest";
import { parseCheckoutOrderTracking, parseShipment } from "./models";

test("parseShipment converts shipment timestamps and tracking events to dates", () => {
	const shipment = parseShipment({
		id: 851,
		order_id: 501,
		snapshot_id: 751,
		provider: "shippo",
		shipment_rate_id: 611,
		provider_shipment_id: "ship_851",
		status: "IN_TRANSIT",
		currency: "USD",
		service_code: "ups_ground",
		service_name: "UPS Ground",
		amount: 12,
		shipping_address_pretty: "Story User, 1 Market St, San Francisco, CA 94105, US",
		tracking_number: "1Z999AA10123456784",
		tracking_url: "https://example.com/track/1Z999AA10123456784",
		label_url: "https://example.com/label/ship_851.pdf",
		purchased_at: "2026-04-05T14:30:00.000Z",
		finalized_at: "2026-04-05T15:00:00.000Z",
		delivered_at: null,
		rates: [],
		packages: [
			{
				id: 771,
				reference: "PKG-1",
				weight_grams: 1200,
				length_cm: 30,
				width_cm: 20,
				height_cm: 10,
			},
		],
		tracking_events: [
			{
				id: 901,
				provider: "shippo",
				provider_event_id: "evt_901",
				status: "IN_TRANSIT",
				tracking_number: "1Z999AA10123456784",
				location: "Oakland, CA",
				description: "Package departed regional facility.",
				occurred_at: "2026-04-06T08:15:00.000Z",
			},
		],
	});

	expect(shipment.purchased_at).toBeInstanceOf(Date);
	expect(shipment.finalized_at).toBeInstanceOf(Date);
	expect(shipment.delivered_at).toBeNull();
	expect(shipment.tracking_events[0]?.occurred_at).toBeInstanceOf(Date);
	expect(shipment.packages[0]?.reference).toBe("PKG-1");
});

test("parseCheckoutOrderTracking keeps the order id and parses every shipment", () => {
	const tracking = parseCheckoutOrderTracking({
		order_id: 501,
		shipments: [
			{
				id: 851,
				order_id: 501,
				snapshot_id: 751,
				provider: "shippo",
				shipment_rate_id: 611,
				provider_shipment_id: "ship_851",
				status: "DELIVERED",
				currency: "USD",
				service_code: "ups_ground",
				service_name: "UPS Ground",
				amount: 12,
				shipping_address_pretty: "Story User, 1 Market St, San Francisco, CA 94105, US",
				tracking_number: "1Z999AA10123456784",
				tracking_url: "https://example.com/track/1Z999AA10123456784",
				label_url: "https://example.com/label/ship_851.pdf",
				purchased_at: "2026-04-05T14:30:00.000Z",
				finalized_at: "2026-04-05T15:00:00.000Z",
				delivered_at: "2026-04-08T18:10:00.000Z",
				rates: [],
				packages: [],
				tracking_events: [],
			},
		],
	});

	expect(tracking.order_id).toBe(501);
	expect(tracking.shipments).toHaveLength(1);
	expect(tracking.shipments[0]?.status).toBe("DELIVERED");
	expect(tracking.shipments[0]?.delivered_at).toBeInstanceOf(Date);
});
