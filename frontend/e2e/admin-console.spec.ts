import { expect, test } from "@playwright/test";
import { seedAndLoginUser, seedTestBrand } from "./admin-helpers";

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

test("admin storefront editor warns before sidebar navigation discards unsaved changes", async ({
	page,
	request,
}) => {
	const now = Date.now();
	await seedAndLoginUser(page, request, {
		email: `admin-dirty-${now}@example.com`,
		username: `admin-dirty-${now}`,
		name: "Dirty Admin",
		role: "admin",
	});

	await page.goto("/admin/storefront");
	await expect(page.getByRole("heading", { name: "Storefront" })).toBeVisible();
	await expect(page.getByRole("button", { name: "Save draft" })).toBeVisible();

	const siteTitleInput = page.getByPlaceholder("Navbar site title");
	await siteTitleInput.fill(`Dirty storefront ${now}`);
	await expect(page.getByText("You have unsaved storefront changes.")).toBeVisible();

	let promptCount = 0;
	page.on("dialog", async (dialog) => {
		promptCount += 1;
		expect(dialog.message()).toContain("You have unsaved storefront changes");
		if (promptCount === 1) {
			await dialog.dismiss();
			return;
		}
		await dialog.accept();
	});

	await page.getByRole("link", { name: "Brands" }).click();
	await expect(page).toHaveURL(/\/admin\/storefront$/);
	await expect(page.getByPlaceholder("Navbar site title")).toHaveValue(`Dirty storefront ${now}`);

	await page.getByRole("link", { name: "Brands" }).click();
	await expect(page).toHaveURL(/\/admin\/brands$/);
	expect(promptCount).toBe(2);
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

test("admin can save storefront drafts and reload the saved values", async ({ page, request }) => {
	const now = Date.now();
	const siteTitle = `E2E Storefront ${now}`;

	await seedAndLoginUser(page, request, {
		email: `admin-storefront-${now}@example.com`,
		username: `admin-storefront-${now}`,
		name: "Storefront Admin",
		role: "admin",
	});

	await page.goto("/admin/storefront");
	await expect(page.getByRole("heading", { name: "Storefront" })).toBeVisible();
	await expect(page.getByRole("button", { name: "Save draft" })).toBeVisible();

	const siteTitleInput = page.getByPlaceholder("Navbar site title");
	await siteTitleInput.fill(siteTitle);
	await page.getByRole("button", { name: "Save draft" }).click();

	await expect(page.getByText("Storefront draft saved.")).toBeVisible();
	await expect(page.getByText("Draft saved", { exact: true })).toBeVisible();

	await page.reload();
	await expect(page.getByRole("heading", { name: "Storefront" })).toBeVisible();
	await expect(page.getByPlaceholder("Navbar site title")).toHaveValue(siteTitle);
});
