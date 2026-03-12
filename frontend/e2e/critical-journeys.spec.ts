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

async function readSummary(
	request: Parameters<typeof test>[0]["request"],
	email?: string
): Promise<TestSummary> {
	const query = email ? `?email=${encodeURIComponent(email)}` : "";
	const response = await request.get(`${apiBaseURL}/__test/summary${query}`);
	expect(response.ok()).toBeTruthy();
	return (await response.json()) as TestSummary;
}

async function seedCartItem(
	request: Parameters<typeof test>[0]["request"],
	input: { email: string; quantity: number; product_variant_id?: number }
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

async function findSeedProductVariantID(
	request: Parameters<typeof test>[0]["request"]
): Promise<number> {
	const response = await request.get(`${apiBaseURL}/api/v1/products?limit=20`);
	expect(response.ok()).toBeTruthy();
	const body = (await response.json()) as {
		data?: Array<{ sku?: string; default_variant_id?: number | null }>;
	};
	const product = body.data?.find((candidate) => candidate.sku === "e2e-product-1");
	expect(product?.default_variant_id).toBeTruthy();
	return Number(product?.default_variant_id);
}

test("sign up, checkout, and persist order/cart state", async ({ page, request }) => {
	const now = Date.now();
	const email = `buyer-${now}@example.com`;
	const password = "supersecret";
	const username = `buyer-${now}`;
	const before = await readSummary(request, email);
	await registerUser(request, { username, email, password, name: "Checkout Buyer" });
	await establishSession(page, email, password);
	await page.goto("/");
	await expect(page).toHaveURL(/\/$/);
	await expect(page.getByRole("link", { name: "View cart" })).toBeVisible();
	await seedCartItem(request, { email, quantity: 1, product_variant_id: 1 });
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

	const after = await readSummary(request, email);
	expect(after.users).toBe(before.users + 1);
	expect(after.orders).toBe(before.orders + 1);
	expect(after.paid_orders).toBe(before.paid_orders);
	expect(after.cart_items).toBe(0);
	expect(after.product_stock).toBe(before.product_stock);
});

test("guest can add to cart and complete checkout", async ({ page, request }) => {
	const now = Date.now();
	const email = `guest-${now}@example.com`;
	const before = await readSummary(request, email);
	const variantID = await findSeedProductVariantID(request);
	const checkoutResponses: Array<{ url: string; status: number; body: string }> = [];
	const profileRequests: string[] = [];
	const consoleErrors: string[] = [];
	const failedRequests: string[] = [];

	page.on("response", async (response) => {
		if (!response.url().includes("/api/v1/checkout/orders")) {
			return;
		}
		checkoutResponses.push({
			url: response.url(),
			status: response.status(),
			body: await response.text(),
		});
	});
	page.on("console", (message) => {
		if (message.type() === "error") {
			consoleErrors.push(message.text());
		}
	});
	page.on("request", (request) => {
		if (request.url().includes("/api/v1/me/")) {
			profileRequests.push(`${request.method()} ${request.url()}`);
		}
	});
	page.on("requestfailed", (request) => {
		failedRequests.push(
			`${request.method()} ${request.url()} :: ${request.failure()?.errorText ?? ""}`
		);
	});

	await page.goto("/");
	const addCartResult = await page.evaluate(
		async ({ apiBaseURL, variantID }) => {
			const response = await fetch(`${apiBaseURL}/api/v1/checkout/cart/items`, {
				method: "POST",
				headers: {
					"Content-Type": "application/json",
				},
				body: JSON.stringify({ product_variant_id: variantID, quantity: 1 }),
				credentials: "include",
			});

			return {
				status: response.status,
				body: await response.text(),
			};
		},
		{ apiBaseURL, variantID }
	);
	expect(addCartResult.status).toBe(200);

	await page.goto("/cart");

	await expect(page).toHaveURL(/\/cart$/);
	await expect(
		page.getByText(
			"Your cart is attached to this browser session. You can still check out as a guest."
		)
	).toBeVisible();
	await expect(page.getByRole("heading", { name: "E2E Running Shoes" })).toBeVisible();
	await page.getByRole("link", { name: "Go to checkout" }).click();

	await expect(page).toHaveURL(/\/checkout$/);
	await page.getByLabel("Email address").fill(email);
	await page.getByRole("button", { name: /Dummy Card Gateway/i }).click();
	await page.getByLabel("Cardholder name").fill("Guest Buyer");
	await page.getByLabel("Card number").fill("4111111111111111");
	await page.getByLabel("Exp month").fill("12");
	await page.getByLabel("Exp year").fill("2030");
	await page.getByRole("button", { name: /Dummy Ground Carrier/i }).click();
	await page.getByLabel("Recipient name").fill("Guest Buyer");
	await page.getByLabel("Address line 1").fill("1 Guest Way");
	await page.getByLabel("City").fill("Austin");
	await page.getByLabel("State/Province").fill("TX");
	await page.getByLabel("Postal code").fill("78701");
	await page.getByLabel("Country").fill("US");
	await page.getByLabel("Service level").selectOption("standard");
	await expect(page.getByText("Save this card to my profile")).toHaveCount(0);
	await expect(page.getByText("Save this address to my profile")).toHaveCount(0);

	await page.getByRole("button", { name: "Place order" }).click();
	await expect
		.poll(() => checkoutResponses.length, {
			message: JSON.stringify(
				{ checkoutResponses, profileRequests, consoleErrors, failedRequests },
				null,
				2
			),
		})
		.toBeGreaterThanOrEqual(2);

	const createOrderResponse = checkoutResponses.find(
		(response) =>
			response.url.endsWith("/api/v1/checkout/orders") ||
			response.url.includes("/api/v1/checkout/orders?")
	);
	expect(
		createOrderResponse,
		JSON.stringify({ checkoutResponses, profileRequests, consoleErrors, failedRequests }, null, 2)
	).toMatchObject({ status: 201 });

	const paymentResponse = checkoutResponses.find((response) =>
		response.url.includes("/payments/authorize")
	);
	expect(
		paymentResponse,
		JSON.stringify({ checkoutResponses, profileRequests, consoleErrors, failedRequests }, null, 2)
	).toMatchObject({ status: 200 });
	expect(profileRequests).toHaveLength(0);

	await expect(
		page.getByText("Order submitted. Keep your confirmation details for reference.")
	).toBeVisible();
	await expect(page.getByText(/Order #\d+/)).toBeVisible();
	await expect(page.getByText("PENDING")).toBeVisible();
	await expect(page.getByText("Guest confirmation token")).toBeVisible();
	await expect(page.getByText(`Order confirmation was sent to ${email}.`)).toBeVisible();

	await page.goto("/cart");
	await expect(page.getByText("Your cart is empty.")).toBeVisible();

	const after = await readSummary(request, email);
	expect(after.users).toBe(0);
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
	await seedCartItem(request, { email, quantity: 1, product_variant_id: 1 });
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

test("guest homepage does not probe profile endpoint", async ({ page }) => {
	const profileRequests: string[] = [];
	page.on("request", (request) => {
		if (request.url().includes("/api/v1/me/")) {
			profileRequests.push(request.url());
		}
	});

	await page.goto("/");
	await expect(page.getByRole("link", { name: "Log In", exact: true })).toBeVisible();
	await expect(page.getByRole("link", { name: "Sign Up", exact: true })).toBeVisible();
	expect(profileRequests).toHaveLength(0);
});
