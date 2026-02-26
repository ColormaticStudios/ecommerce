import { expect, test } from "@playwright/test";

// Test-only browser E2E coverage for critical user journeys.
// These specs depend on the test API helpers exposed under `/__test/*` by
// `cmd/e2e-server` and are not part of production app behavior.
const apiBaseURL = process.env.PUBLIC_API_BASE_URL || "http://127.0.0.1:3001";

type TestSummary = {
	users: number;
	orders: number;
	paid_orders: number;
	cart_items: number;
	product_stock: number;
};

async function registerUser(
	request: Parameters<typeof test>[0]["request"],
	input: { username: string; email: string; password: string; name?: string }
): Promise<void> {
	const response = await request.post(`${apiBaseURL}/api/v1/auth/register`, {
		data: input,
	});
	expect(response.ok()).toBeTruthy();
}

async function establishSession(
	page: Parameters<typeof test>[0]["page"],
	email: string,
	password: string
): Promise<void> {
	void password;
	await page.goto(`${apiBaseURL}/__test/login?email=${encodeURIComponent(email)}`);
}

async function readSummary(request: Parameters<typeof test>[0]["request"]): Promise<TestSummary> {
	const response = await request.get(`${apiBaseURL}/__test/summary`);
	expect(response.ok()).toBeTruthy();
	return (await response.json()) as TestSummary;
}

async function seedCartItem(
	request: Parameters<typeof test>[0]["request"],
	input: { email: string; quantity: number; product_id?: number }
): Promise<void> {
	const response = await request.post(`${apiBaseURL}/__test/cart-item`, { data: input });
	expect(response.ok()).toBeTruthy();
}

async function seedSavedCheckoutData(
	request: Parameters<typeof test>[0]["request"],
	email: string
): Promise<void> {
	const response = await request.post(`${apiBaseURL}/__test/saved-checkout-data`, {
		data: { email },
	});
	expect(response.ok()).toBeTruthy();
}

test("sign up, checkout, and persist order/cart state", async ({ page, request }) => {
	const before = await readSummary(request);

	const now = Date.now();
	const email = `buyer-${now}@example.com`;
	const password = "supersecret";
	const username = `buyer-${now}`;
	await registerUser(request, { username, email, password, name: "Checkout Buyer" });
	await establishSession(page, email, password);
	await page.goto("/");
	await expect(page).toHaveURL(/\/$/);
	await expect(page.getByRole("link", { name: "View cart" })).toBeVisible();
	await seedCartItem(request, { email, quantity: 1, product_id: 1 });
	await seedSavedCheckoutData(request, email);

	await page.goto("/cart");

	await expect(page).toHaveURL(/\/cart$/);
	await page.getByRole("link", { name: "Go to checkout" }).click();
	await expect(page).toHaveURL(/\/checkout$/);
	await page.getByRole("button", { name: /Dummy Card Gateway/i }).click();
	await page.getByLabel("Use a saved card").selectOption({ index: 1 });
	await page.getByRole("button", { name: /Dummy Ground Carrier/i }).click();
	await page.getByLabel("Use a saved address").selectOption({ index: 1 });
	await page.getByLabel("State/Province").fill("TX");

	await page.getByRole("button", { name: "Place order" }).click();

	await expect(page).toHaveURL(/\/orders$/, { timeout: 15_000 });
	await expect(page.getByText("Order placed successfully.")).toBeVisible();
	await expect(page.getByText(/Order #\d+/)).toBeVisible();
	await expect(page.getByText("PENDING", { exact: true })).toBeVisible();

	await page.goto("/cart");
	await expect(page.getByText("Your cart is empty.")).toBeVisible();

	const after = await readSummary(request);
	expect(after.users).toBe(before.users + 1);
	expect(after.orders).toBe(before.orders + 1);
	expect(after.paid_orders).toBe(before.paid_orders);
	expect(after.cart_items).toBe(0);
	expect(after.product_stock).toBe(before.product_stock);
});

test("invalid data paths show user-facing errors", async ({ page, request }) => {
	const now = Date.now();
	const email = `invalid-${now}@example.com`;
	const password = "supersecret";
	const username = `invalid-${now}`;

	await registerUser(request, { username, email, password });
	await establishSession(page, email, password);
	await page.goto("/");
	await expect(page).toHaveURL(/\/$/);
	await seedCartItem(request, { email, quantity: 1, product_id: 1 });
	await page.goto("/cart");
	await page.getByRole("link", { name: "Go to checkout" }).click();
	await expect(page).toHaveURL(/\/checkout$/);

	await page.getByRole("button", { name: "Place order" }).click();
	await expect(
		page.getByText("Select a payment and shipping option before requesting an estimate.")
	).toBeVisible();

	const invalidLoginResponse = await request.post(`${apiBaseURL}/api/v1/auth/login`, {
		data: {
			email,
			password: "wrong-password",
		},
	});
	expect(invalidLoginResponse.status()).toBe(401);
});
