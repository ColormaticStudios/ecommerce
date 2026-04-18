import { describe, expect, it } from "vitest";
import { formatOrderStatusLabel, getOrderStatusTone } from "./order-status";

describe("getOrderStatusTone", () => {
	it("matches the admin order badge tones", () => {
		expect(getOrderStatusTone("PENDING")).toBe("warning");
		expect(getOrderStatusTone("PAID")).toBe("success");
		expect(getOrderStatusTone("SHIPPED")).toBe("info");
		expect(getOrderStatusTone("DELIVERED")).toBe("success");
		expect(getOrderStatusTone("FAILED")).toBe("danger");
		expect(getOrderStatusTone("CANCELLED")).toBe("neutral");
		expect(getOrderStatusTone("REFUNDED")).toBe("neutral");
	});
});

describe("formatOrderStatusLabel", () => {
	it("formats order statuses for customer-facing copy", () => {
		expect(formatOrderStatusLabel("PENDING")).toBe("Pending");
		expect(formatOrderStatusLabel("REFUNDED")).toBe("Refunded");
	});
});
