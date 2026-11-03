import { expect, test } from "@playwright/test";
import { apiBaseURL, seedAndLoginUser, seedTestUser } from "./admin-helpers";
import type { components } from "../src/lib/api/generated/openapi";

test.use({ timezoneId: "Europe/Paris" });
test.beforeEach(({ page }) => {
	page.on("pageerror", (error) => console.error("Search journey browser error:", error.message));
});

test("consent does not rerun or replace search; a pending click remains attributable to cart", async ({
	page,
	request,
}) => {
	await page.addInitScript(() => {
		const fetch = window.fetch.bind(window);
		window.fetch = (input, options) => {
			const url = input instanceof Request ? input.url : String(input);
			if (new URL(url).pathname.endsWith("/api/v1/search/events") && options?.method === "POST") {
				const clicks = JSON.parse(sessionStorage.getItem("e2e.search.clicks") ?? "[]");
				clicks.push(JSON.parse(String(options.body)));
				sessionStorage.setItem("e2e.search.clicks", JSON.stringify(clicks));
				sessionStorage.setItem("e2e.search.keepalive", String(options.keepalive));
				if (clicks.length === 1) {
					// A paused Chromium route can stall unrelated network traffic.
					// Hold the analytics promise while replay and cart use real HTTP.
					return new Promise<Response>((resolve) => {
						Reflect.set(window, "releaseHeldSearchEvent", () => {
							resolve(new Response("{}", { status: 200 }));
						});
					});
				}
			}
			return fetch(input, options);
		};
	});
	const stamp = Date.now();
	const analyticsAdmin = await seedTestUser(request, {
		email: `search-analytics-${stamp}@example.com`,
		username: `search-analytics-${stamp}`,
		role: "admin",
	});
	expect(
		(await request.get(`${apiBaseURL}/__test/login?email=${analyticsAdmin.email}`)).ok()
	).toBeTruthy();
	const readAnalytics = async () => {
		const response = await request.get(`${apiBaseURL}/api/v1/admin/search/analytics?days=7`);
		expect(response.ok()).toBeTruthy();
		return (await response.json()) as components["schemas"]["SearchAnalytics"];
	};
	const analyticsBefore = await readAnalytics();
	await seedAndLoginUser(page, request, {
		email: `search-buyer-${stamp}@example.com`,
		username: `search-buyer-${stamp}`,
	});
	const impressions: Record<string, unknown>[] = [];
	page.on("request", (request) => {
		if (new URL(request.url()).pathname.endsWith("/search/impressions"))
			impressions.push(request.postDataJSON());
	});
	await page.goto("/search?q=running");
	const product = page.locator('a[href="/product/1"], a[href^="/product/1?"]');
	await expect(product).toBeVisible();
	const before = await page.locator('a[href^="/product/"]').allTextContents();
	expect(impressions).toHaveLength(0);
	const proofResponse = page.waitForResponse(
		(response) => new URL(response.url()).pathname.endsWith("/search/proofs") && response.ok()
	);
	const impression = page.waitForResponse(
		(response) => new URL(response.url()).pathname.endsWith("/search/impressions") && response.ok()
	);
	await page.getByRole("checkbox", { name: /Allow anonymous search activity/ }).check();
	expect((await proofResponse).headers()["cache-control"]).toBe("private, no-store");
	const exposure = await (await impression).json();
	expect(Object.keys(exposure).sort()).toEqual(["impression_id", "session_token"]);
	expect(impressions).toHaveLength(1);
	expect(impressions[0]).toHaveProperty("result_token");
	expect(impressions[0]).not.toHaveProperty("filters");
	expect(await page.locator('a[href^="/product/"]').allTextContents()).toEqual(before);
	const recordedClicks = () =>
		page.evaluate(
			() =>
				JSON.parse(sessionStorage.getItem("e2e.search.clicks") ?? "[]") as Record<string, unknown>[]
		);
	try {
		await product.click();
		await expect(page).toHaveURL((url) => url.pathname === "/product/1");
		expect(await recordedClicks()).toHaveLength(1);
		expect(await page.evaluate(() => sessionStorage.getItem("e2e.search.keepalive"))).toBe("true");
		const cart = page.waitForRequest(
			(request) =>
				request.method() === "POST" && new URL(request.url()).pathname.endsWith("/cart/items")
		);
		const cartResponse = page.waitForResponse(
			(response) =>
				response.request().method() === "POST" &&
				new URL(response.url()).pathname.endsWith("/cart/items")
		);
		await page.getByRole("button", { name: /Add to cart/ }).click();
		const input = (await cart).postDataJSON();
		const clicks = await recordedClicks();
		expect(clicks).toHaveLength(2);
		expect(clicks[1].event_id).toBe(clicks[0].event_id);
		expect(input.search_click_id).toBeTruthy();
		expect(input.search_session_token).toBe(exposure.session_token);
		expect((await cartResponse).ok()).toBeTruthy();
		await expect(page.getByText("Added to cart.", { exact: true })).toBeVisible();
		const analyticsAfter = await readAnalytics();
		expect(analyticsAfter.clicks).toBe(analyticsBefore.clicks + 1);
		expect(analyticsAfter.add_to_carts).toBe(analyticsBefore.add_to_carts + 1);
	} finally {
		await page.evaluate(() => {
			const release = Reflect.get(window, "releaseHeldSearchEvent");
			if (typeof release === "function") release();
		});
	}
});

test("suggestions and corrections share an accessible keyboard combobox", async ({ page }) => {
	await page.route("**/api/v1/search/suggestions?*", (route) =>
		route.fulfill({
			json: {
				suggestions: ["running", "running shoes"],
				corrections: ["shoes"],
			},
		})
	);
	await page.goto("/search");
	await page.waitForLoadState("networkidle");
	const input = page.getByRole("combobox", { name: "Search products" });
	await input.fill("run");
	await expect(input).toHaveAttribute("aria-expanded", "true");
	await input.press("ArrowDown");
	let active = await input.getAttribute("aria-activedescendant");
	await expect(page.locator(`[id="${active}"]`)).toHaveText("running");
	await input.press("End");
	active = await input.getAttribute("aria-activedescendant");
	await expect(page.locator(`[id="${active}"]`)).toHaveText("Did you mean shoes");
	await input.press("Escape");
	await expect(input).toHaveAttribute("aria-expanded", "false");
	await expect(input).toHaveValue("run");
	await input.press("ArrowUp");
	await input.press("Enter");
	await expect(page).toHaveURL(/q=shoes/);
	await expect(page.locator('a[href="/product/1"], a[href^="/product/1?"]')).toBeVisible();
	await expect(page.getByText("Popular searches", { exact: true })).toHaveCount(0);
	await expect(page.getByText("Trending searches", { exact: true })).toHaveCount(0);
});

test("synonym edits affect real storefront search and stay in one Search section", async ({
	page,
	request,
}) => {
	const stamp = Date.now();
	await seedAndLoginUser(page, request, {
		email: `synonym-admin-${stamp}@example.com`,
		username: `synonym-admin-${stamp}`,
		role: "admin",
	});
	await page.goto("/admin/search/synonyms");
	await page.waitForLoadState("networkidle");
	const tabs = page.getByRole("navigation", { name: "Search sections" });
	await expect(tabs.getByRole("link", { name: "Synonyms", exact: true })).toHaveAttribute(
		"aria-current",
		"page"
	);
	const name = `Running synonym ${stamp}`;
	const source = `sprint${stamp}`;
	await page.getByRole("textbox", { name: "Name", exact: true }).fill(name);
	await page.getByRole("combobox", { name: "Direction" }).selectOption("uni");
	await page.getByRole("textbox", { name: "Terms, one per line" }).fill(`${source}\nrunning`);
	await page.getByRole("button", { name: "Create", exact: true }).click();
	await expect(page.getByText("Search settings saved.", { exact: true })).toBeVisible();
	try {
		await page.goto(`/search?q=${source}`);
		await expect(page.locator('a[href="/product/1"], a[href^="/product/1?"]')).toBeVisible();
	} finally {
		await page.goto("/admin/search/synonyms");
		await page.waitForLoadState("networkidle");
		await page.getByRole("button", { name: new RegExp(name) }).click();
		page.once("dialog", (dialog) => dialog.accept());
		await page.getByRole("button", { name: "Delete", exact: true }).click();
	}
});

test("merchandising uses admin product lookup, local schedules and multi-stock price preview", async ({
	page,
	request,
}) => {
	const stamp = Date.now();
	await seedAndLoginUser(page, request, {
		email: `merch-admin-${stamp}@example.com`,
		username: `merch-admin-${stamp}`,
		role: "admin",
	});
	await page.goto("/admin/search/merchandising");
	await expect(
		page.getByText(/Times use your browser timezone: Europe\/Paris/).first()
	).toBeVisible();
	const name = `Running campaign ${stamp}`;
	await page.getByRole("textbox", { name: "Rule name" }).fill(name);
	await page
		.getByRole("textbox", { name: "Query (blank matches all)", exact: true })
		.fill("running");
	await page.getByLabel("Starts at (local time, optional)").fill("2020-10-10T12:30");
	await page.getByLabel("Ends at (local time, optional)").fill("2030-10-10T12:30");
	await page.getByPlaceholder("Search products", { exact: true }).fill("running");
	const lookup = page.waitForRequest(
		(request) => request.url().includes("/api/v1/admin/products?") && request.method() === "GET"
	);
	await page.getByRole("button", { name: "Search", exact: true }).click();
	expect(new URL((await lookup).url()).searchParams.get("q")).toBe("running");
	await page.getByRole("button", { name: /E2E Running Shoes Add/ }).click();
	const save = page.waitForRequest(
		(request) =>
			new URL(request.url()).pathname.endsWith("/search/merchandising-rules") &&
			request.method() === "POST"
	);
	await page.getByRole("button", { name: /Create rule/ }).click();
	expect((await save).postDataJSON().starts_at).toBe("2020-10-10T10:30:00.000Z");
	await expect(page.getByText("Rule saved.", { exact: true })).toBeVisible();
	try {
		await page.getByRole("textbox", { name: "Search query", exact: true }).fill("running");
		await page.getByRole("spinbutton", { name: "Minimum price" }).fill("80");
		await page.getByRole("spinbutton", { name: "Maximum price" }).fill("100");
		const stock = page.getByRole("combobox", { name: "Search stock to add" });
		await stock.click();
		await stock.press("ArrowDown");
		await stock.press("Enter");
		await stock.click();
		await stock.press("ArrowDown");
		await stock.press("Enter");
		const preview = page.waitForRequest((request) =>
			new URL(request.url()).pathname.endsWith("/admin/search/preview")
		);
		await page.getByRole("button", { name: "Preview saved rules", exact: true }).click();
		expect((await preview).postDataJSON().filters).toMatchObject({
			q: "running",
			has_variant_stock: [true, false],
			price_range: ["80:100"],
		});
		await expect(page.getByText("Preview uses saved rules.", { exact: true })).toBeVisible();
	} finally {
		await page.getByRole("button", { name: /Delete rule/ }).click();
		await page
			.getByRole("alertdialog")
			.getByRole("button", { name: "Delete rule", exact: true })
			.click();
		await expect(page.getByText("Rule deleted.", { exact: true })).toBeVisible();
	}
});

test("degraded browsing preserves removable filters, disables advanced controls and never records an impression", async ({
	page,
	request,
}) => {
	let impressions = 0;
	page.on("request", (request) => {
		if (new URL(request.url()).pathname.endsWith("/search/impressions")) impressions++;
	});
	const unavailable = await request.post(`${apiBaseURL}/__test/search-index-availability`, {
		data: { available: false },
	});
	expect(unavailable.ok()).toBeTruthy();
	try {
		await page.goto("/search?q=running&has_variant_stock=true&has_variant_stock=false");
		await page.waitForLoadState("networkidle");
		await expect(
			page.getByRole("status").filter({ hasText: "Search is temporarily unavailable." })
		).toBeVisible();
		await expect(page.locator('a[href="/product/1"], a[href^="/product/1?"]')).toBeVisible();
		await expect(page.getByRole("combobox", { name: "Search products" })).toBeDisabled();
		await page.getByRole("checkbox", { name: /Allow anonymous search activity/ }).check();
		await expect(page.getByRole("checkbox", { name: /Out of stock/ })).toBeChecked();
		await page.getByRole("checkbox", { name: /Out of stock/ }).click();
		await expect(page).toHaveURL(/has_variant_stock=true/);
		await expect(page.getByRole("checkbox", { name: /Out of stock/ })).not.toBeChecked();
		expect(new URL(page.url()).searchParams.getAll("has_variant_stock")).toEqual(["true"]);
		expect(impressions).toBe(0);
	} finally {
		const restored = await request.post(`${apiBaseURL}/__test/search-index-availability`, {
			data: { available: true },
		});
		expect(restored.ok()).toBeTruthy();
	}
});

test("canonical search transport isolates negotiation and authenticated responses", async ({
	page,
	request,
}) => {
	const negotiation = await request.get("/search?q=running", { maxRedirects: 0 });
	expect(negotiation.status()).toBe(307);
	expect(negotiation.headers()["cache-control"]).toBe("private, no-store");
	const dataNegotiation = await request.get("/search/__data.json?q=running", { maxRedirects: 0 });
	expect(dataNegotiation.status()).toBe(200);
	expect(dataNegotiation.headers()["cache-control"]).toBe("private, no-store");
	expect(await dataNegotiation.json()).toEqual({
		type: "redirect",
		location: negotiation.headers().location,
	});
	const canonical = new URL(negotiation.headers().location, negotiation.url());
	expect(canonical.pathname).toBe("/search");
	expect(canonical.searchParams.get("q")).toBe("running");
	expect(canonical.searchParams.get("locale")).toBe("en-US");
	expect(canonical.searchParams.has("market")).toBeTruthy();
	const publicPage = await request.get(canonical.toString(), {
		headers: { "Accept-Language": "de-DE,de;q=0.9", Cookie: "locale=invalid; market=US" },
	});
	expect(publicPage.status()).toBe(200);
	expect(publicPage.headers()["cache-control"]).toContain("public,");
	expect(publicPage.headers()["set-cookie"]).toBeUndefined();
	expect(publicPage.headers().vary ?? "").not.toMatch(
		/cookie|authorization|accept-language|x-locale|x-market/i
	);
	const stamp = Date.now();
	await seedAndLoginUser(page, request, {
		email: `canonical-cache-${stamp}@example.com`,
		username: `canonical-cache-${stamp}`,
	});
	const privatePage = await page.goto(canonical.toString());
	expect(privatePage?.status()).toBe(200);
	expect(privatePage?.headers()["cache-control"]).toBe("private, no-store");
});
