import type { Meta, StoryObj } from "@storybook/sveltekit";
import { expect, userEvent, within } from "storybook/test";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { renderRouteStory } from "$lib/storybook/render";
import SearchCombobox from "./SearchCombobox.svelte";

export default { title: "Storefront/SearchCombobox", component: RouteStoryHarness } satisfies Meta;
export const SuggestionsAndCorrections: StoryObj = {
	render: () =>
		renderRouteStory({
			component: SearchCombobox,
			componentProps: {
				value: "jac",
				label: "Search products",
				suggestions: ["jacket", "jackets"],
				corrections: ["jackets"],
				oninput: () => {},
				onselect: () => {},
			},
		}),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const input = canvas.getByRole("combobox", { name: "Search products" });
		await userEvent.click(input);
		await expect(canvas.getByRole("option", { name: /^jacket$/ })).toBeVisible();
		await expect(canvas.getByRole("option", { name: "Did you mean jackets" })).toBeVisible();
		await userEvent.keyboard("{ArrowDown}{End}{Enter}");
		await expect(input).toHaveValue("jackets");
	},
};
