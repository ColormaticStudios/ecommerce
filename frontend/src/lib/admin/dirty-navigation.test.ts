import assert from "node:assert/strict";
import test from "node:test";
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

	assert.equal(controller.shouldBlockNavigation("/admin/brands", "/admin/products"), true);
	assert.equal(controller.confirmNavigation(), true);
	assert.deepEqual(prompts, [
		"You have unsaved brand changes. Leave this section and discard them?",
	]);

	controller.allowNextNavigation("/admin/products");
	assert.equal(controller.shouldBlockNavigation("/admin/brands", "/admin/products"), false);
	assert.equal(controller.shouldBlockNavigation("/admin/products", "/admin/users"), true);
});

test("ignores unchanged destinations and clears its dirty state when the page unregisters", () => {
	const controller = createAdminDirtyNavigationController(() => true);
	const token = Symbol("product");

	controller.update(token, { dirty: true });

	assert.equal(controller.shouldBlockNavigation("/admin/products", "/admin/products"), false);
	assert.equal(controller.dirty, true);

	controller.clear(token);

	assert.equal(controller.dirty, false);
	assert.equal(controller.shouldBlockNavigation("/admin/products", "/admin/brands"), false);
});

test("normalizes route targets without hashes for prompt comparisons", () => {
	const target = toAdminNavigationTarget(
		new URL("https://example.com/admin/storefront?preview=draft#footer")
	);

	assert.equal(target, "/admin/storefront?preview=draft");
});
