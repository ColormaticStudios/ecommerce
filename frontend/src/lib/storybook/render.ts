import type { API } from "$lib/api";
import type { UserModel } from "$lib/models";
import type { LocalizationRuntime } from "$lib/localization/runtime";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";

interface RenderRouteOptions {
	component: unknown;
	componentProps?: Record<string, unknown>;
	api?: Partial<API>;
	user?: UserModel | null;
	localization?: LocalizationRuntime;
}

export function renderRouteStory(options: RenderRouteOptions): never {
	return {
		Component: RouteStoryHarness,
		props: options,
	} as never;
}
