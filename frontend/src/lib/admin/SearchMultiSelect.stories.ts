import type { Meta, StoryObj } from "@storybook/sveltekit";
import { expect, userEvent, within } from "storybook/test";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { renderRouteStory } from "$lib/storybook/render";
import SearchMultiSelect from "./SearchMultiSelect.svelte";

export default { title: "Admin/SearchMultiSelect", component: RouteStoryHarness } satisfies Meta;
export const LocalizedLabels: StoryObj = {
	render: () =>
		renderRouteStory({
			component: SearchMultiSelect,
			componentProps: {
				label: "Kategorien",
				searchLabel: "Kategorien durchsuchen",
				options: [{ label: "Bekleidung", value: "clothing" }],
				values: [],
			},
		}),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("Kategorien", { exact: true })).toBeVisible();
		const input = canvas.getByRole("combobox", { name: "Kategorien durchsuchen" });
		await userEvent.click(input);
		await userEvent.keyboard("{ArrowDown}{Enter}");
		await expect(canvas.getByRole("button", { name: "Remove Bekleidung" })).toBeVisible();
	},
};
