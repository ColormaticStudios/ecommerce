import type { Meta, StoryObj } from "@storybook/sveltekit";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import AdminSearchTabs from "$lib/admin/AdminSearchTabs.svelte";
import { renderRouteStory } from "$lib/storybook/render";

const meta = { title: "Routes/Admin/Search/Sections", component: RouteStoryHarness } satisfies Meta;
export default meta;
type Story = StoryObj;
export const Sections: Story = {
	render: () => renderRouteStory({ component: AdminSearchTabs, componentProps: {} }),
};
