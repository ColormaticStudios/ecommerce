import { expect, test } from "vitest";
import { createAdminDirtyNavigationController, toAdminNavigationTarget } from "./dirty-navigation";

test("blocks dirty cross-route navigation until the destination is explicitly allowed", () => {
	const prompts: string[] = [];
	const controller = createAdminDirtyNavigationController((message) => {
		prompts.push(message);
		return true;
	});
	const token = Symbol("brands");

	controller.update(token, {
		dirty: true,
		message: "You have unsaved brand changes. Leave this section and discard them?",
	});

	expect(controller.shouldBlockNavigation("/admin/brands", "/admin/products")).toBe(true);
	expect(controller.confirmNavigation()).toBe(true);
	expect(prompts).toEqual(["You have unsaved brand changes. Leave this section and discard them?"]);

	controller.allowNextNavigation("/admin/products");
	expect(controller.shouldBlockNavigation("/admin/brands", "/admin/products")).toBe(false);
	expect(controller.shouldBlockNavigation("/admin/products", "/admin/users")).toBe(true);
});

test("ignores unchanged destinations and clears its dirty state when the page unregisters", () => {
	const controller = createAdminDirtyNavigationController(() => true);
	const token = Symbol("product");

	controller.update(token, { dirty: true });

	expect(controller.shouldBlockNavigation("/admin/products", "/admin/products")).toBe(false);
	expect(controller.dirty).toBe(true);

	controller.clear(token);

	expect(controller.dirty).toBe(false);
	expect(controller.shouldBlockNavigation("/admin/products", "/admin/brands")).toBe(false);
});

test("normalizes route targets without hashes for prompt comparisons", () => {
	const target = toAdminNavigationTarget(
		new URL("https://example.com/admin/storefront?preview=draft#footer")
	);

	expect(target).toBe("/admin/storefront?preview=draft");
});
