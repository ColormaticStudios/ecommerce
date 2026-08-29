import { expect, test } from "@playwright/test";
import { seedAndLoginUser, seedTestBrand, seedTestUser } from "./admin-helpers";

test("guest and customer users are denied admin console access", async ({ page, request }) => {
	await page.goto("/admin/products");
	await expect(page.getByText("Access denied.")).toBeVisible();
	await expect(
		page.getByText("You must be signed in to an admin account to access the admin console.")
	).toBeVisible();

	const now = Date.now();
	await seedAndLoginUser(page, request, {
		email: `customer-${now}@example.com`,
		username: `customer-${now}`,
		name: "Customer Viewer",
		role: "customer",
	});

	await page.goto("/admin/products");
	await expect(page.getByText("Access denied.")).toBeVisible();
	await expect(page.getByText("Contact an administrator if you need access.")).toBeVisible();
});

test("admin can navigate between sections from the mobile drawer without localization diagnostics", async ({
	page,
	request,
}) => {
	const consoleDiagnostics: string[] = [];
	page.on("console", (message) => {
		if (message.type() === "warning" || message.type() === "error") {
			consoleDiagnostics.push(message.text());
		}
	});

	const now = Date.now();
	await page.setViewportSize({ width: 755, height: 800 });
	await seedAndLoginUser(page, request, {
		email: `admin-navigation-${now}@example.com`,
		username: `admin-navigation-${now}`,
		name: "Navigation Admin",
		role: "admin",
	});
	await seedTestUser(request, {
		email: `localization-customer-${now}@example.com`,
		username: `localization-customer-${now}`,
		name: "Localization Customer",
		role: "customer",
	});

	await page.goto("/admin/products");
	await expect(page.getByRole("heading", { level: 1, name: "Products" })).toBeVisible();
	const openDrawerButton = page.getByRole("button", { name: "Open admin sections" });
	await expect(openDrawerButton).toBeEnabled();
	await openDrawerButton.click();
	const drawer = page.getByRole("dialog", { name: "Admin section drawer" });
	await expect(drawer, `Browser console: ${consoleDiagnostics.join(" | ")}`).toBeVisible();
	await drawer
		.getByRole("link", {
			name: "Brands",
		})
		.click();
	await expect(page).toHaveURL(/\/admin\/brands$/);
	await expect(page.getByRole("heading", { level: 1, name: "Brands" })).toBeVisible();

	await page.getByRole("button", { name: "Open admin sections" }).click();
	await page
		.getByRole("dialog", { name: "Admin section drawer" })
		.getByRole("link", {
			name: "Categories",
		})
		.click();
	await expect(page).toHaveURL(/\/admin\/categories$/);
	await expect(page.getByRole("heading", { level: 1, name: "Categories" })).toBeVisible();

	await page.getByRole("button", { name: "Open admin sections" }).click();
	await page
		.getByRole("dialog", { name: "Admin section drawer" })
		.getByRole("link", { name: "Translations" })
		.click();
	await expect(page).toHaveURL(/\/admin\/localization$/);
	await expect(page.getByRole("heading", { level: 1, name: "Translations" })).toBeVisible();
	await expect(page.getByRole("heading", { level: 2, name: "Translation queue" })).toBeVisible();
	const assigneePicker = page.getByRole("combobox", { name: "Filter by assignee" });
	await assigneePicker.fill("Navigation Admin");
	await expect(page.getByRole("option", { name: /Navigation Admin/ })).toBeVisible();
	await page.getByRole("option", { name: /Navigation Admin/ }).click();
	await expect(assigneePicker).toHaveValue("Navigation Admin");
	await assigneePicker.fill("Localization Customer");
	await expect(page.getByText("No eligible people found.")).toBeVisible();

	await expect(page.getByText(/Page 1 of \d+ \(\d+ total\)/)).toBeVisible();
	await page.getByRole("button", { name: "Next" }).click();
	await expect(page.getByText(/Page 2 of \d+ \(\d+ total\)/)).toBeVisible();
	const translationSearch = page.getByRole("textbox", { name: "Search translation keys" });
	await translationSearch.fill("checkout.empty_cart_prefix");
	await translationSearch.press("Enter");
	await expect(page.getByText("empty_cart_prefix", { exact: true })).toBeVisible();
	await expect(page.getByText("Page 1 of 1 (1 total)", { exact: true })).toBeVisible();

	await page.getByRole("tab", { name: "Rollout & health" }).click();
	await expect(
		page.getByRole("heading", { level: 2, name: "Locale rollout controls" })
	).toBeVisible();
	await expect(page.getByRole("heading", { level: 2, name: "Localization health" })).toBeVisible();
	await expect(page.getByRole("button", { name: "Save rollout controls" })).toBeVisible();

	await expect(page.locator("body")).not.toContainText(/⟦[^⟧]+⟧/);
	expect(consoleDiagnostics).toEqual([]);
});

test("admin can search and reopen a seeded brand after reload", async ({ page, request }) => {
	const now = Date.now();
	const brandName = `E2E Brand ${now}`;
	const brandSlug = `e2e-brand-${now}`;
	const brandDescription = "Created from the admin Playwright flow.";

	await seedAndLoginUser(page, request, {
		email: `admin-brand-${now}@example.com`,
		username: `admin-brand-${now}`,
		name: "Brand Admin",
		role: "admin",
	});
	await seedTestBrand(request, {
		name: brandName,
		slug: brandSlug,
		description: brandDescription,
	});

	await page.goto("/admin/brands");
	await expect(page.getByRole("heading", { level: 1, name: "Brands" })).toBeVisible();

	await page.reload();
	await expect(page.getByRole("heading", { level: 1, name: "Brands" })).toBeVisible();

	const searchInput = page.getByPlaceholder("Search brands");
	await searchInput.fill(brandName);
	await page.getByRole("button", { name: "Search" }).click();
	await expect(page.getByRole("button", { name: new RegExp(brandName) })).toBeVisible();
	await page.getByRole("button", { name: new RegExp(brandName) }).click();

	await expect(page.getByLabel("Name")).toHaveValue(brandName);
	await expect(page.getByLabel("Slug")).toHaveValue(brandSlug);
	await expect(page.getByLabel("Description")).toHaveValue(brandDescription);
});

test("admin can create a brand from the brands console", async ({ page, request }) => {
	const now = Date.now();
	const brandName = `Created Brand ${now}`;
	const brandSlug = `created-brand-${now}`;
	const brandDescription = "Saved from the admin brands console.";

	await seedAndLoginUser(page, request, {
		email: `admin-brand-create-${now}@example.com`,
		username: `admin-brand-create-${now}`,
		name: "Brand Creator",
		role: "admin",
	});

	await page.goto("/admin/brands");
	await expect(page.getByRole("heading", { level: 1, name: "Brands" })).toBeVisible();
	await page.waitForResponse((response) => {
		return response.request().method() === "GET" && response.url().endsWith("/api/v1/admin/brands");
	});

	await page.getByLabel("Name").fill(brandName);
	await page.getByLabel("Slug").fill(brandSlug);
	await page.getByLabel("Description").fill(brandDescription);

	const createRequestPromise = page.waitForRequest((req) => {
		return req.method() === "POST" && req.url().endsWith("/api/v1/admin/brands");
	});

	await page.getByRole("button", { name: "Create brand" }).click();
	await createRequestPromise;

	await expect(page.getByText("Brand created.")).toBeVisible();
	await expect(page.getByLabel("Name")).toHaveValue(brandName);
	await expect(page.getByLabel("Slug")).toHaveValue(brandSlug);
	await expect(page.getByLabel("Description")).toHaveValue(brandDescription);
});
