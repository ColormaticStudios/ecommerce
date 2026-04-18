import { afterEach, expect, test, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	serverIsAuthenticated: vi.fn(),
	serverRequest: vi.fn(),
	parseOrder: vi.fn(),
	parseCheckoutOrderTracking: vi.fn(),
}));

vi.mock("$lib/server/api", () => ({
	serverIsAuthenticated: mocks.serverIsAuthenticated,
	serverRequest: mocks.serverRequest,
}));

vi.mock("$lib/models", () => ({
	parseOrder: mocks.parseOrder,
	parseCheckoutOrderTracking: mocks.parseCheckoutOrderTracking,
}));

import { load } from "./+page.server";

afterEach(() => {
	vi.clearAllMocks();
});

function createEvent(id: string) {
	return {
		params: { id },
		request: new Request("http://localhost/orders/" + id),
	};
}

test("keeps authenticated users out of the signed-out fallback for invalid ids", async () => {
	mocks.serverIsAuthenticated.mockResolvedValue(true);

	const result = await load(createEvent("0") as never);

	expect(mocks.serverRequest).not.toHaveBeenCalled();
	expect(result).toMatchObject({
		isAuthenticated: true,
		order: null,
		errorMessage: "Order not found.",
		trackingErrorMessage: "",
		shipments: [],
	});
});

test("keeps authenticated users in the error state when unexpected parsing fails", async () => {
	mocks.serverIsAuthenticated.mockResolvedValue(true);
	mocks.serverRequest
		.mockResolvedValueOnce({})
		.mockResolvedValueOnce({ order_id: 42, shipments: [] });
	mocks.parseOrder.mockImplementation(() => {
		throw new Error("invalid order payload");
	});

	const result = await load(createEvent("42") as never);

	expect(result).toMatchObject({
		isAuthenticated: true,
		order: null,
		errorMessage: "Unable to load this order.",
	});
});
