import { expect, test, vi } from "vitest";
import { claimGuestOrder } from "./orders";
import { makeOrder } from "$lib/storybook/factories";

test("claimGuestOrder posts the claim payload and parses the returned order", async () => {
	const request = vi.fn().mockResolvedValue({
		message: "Order claimed.",
		order: {
			...makeOrder({
				id: 712,
				guest_email: "guest@example.com",
			}),
			created_at: "2026-04-08T12:00:00.000Z",
			updated_at: "2026-04-08T12:00:00.000Z",
			deleted_at: null,
			items: [],
		},
	});

	const result = await claimGuestOrder(request, {
		email: "guest@example.com",
		confirmation_token: "token-123",
	});

	expect(request).toHaveBeenCalledWith("POST", "/me/orders/claim", {
		email: "guest@example.com",
		confirmation_token: "token-123",
	});
	expect(result.message).toBe("Order claimed.");
	expect(result.order.id).toBe(712);
	expect(result.order.created_at).toBeInstanceOf(Date);
});
