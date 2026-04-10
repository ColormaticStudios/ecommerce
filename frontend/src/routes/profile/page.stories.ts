import type { Meta, StoryObj } from "@storybook/sveltekit";
import type { ComponentProps } from "svelte";
import RouteStoryHarness from "$lib/storybook/RouteStoryHarness.svelte";
import { makeSavedAddress, makeSavedPaymentMethod, makeUser } from "$lib/storybook/factories";
import { createApiStub } from "$lib/storybook/api";
import { makeRouteLayoutData } from "$lib/storybook/layout";
import { renderRouteStory } from "$lib/storybook/render";
import ProfilePage from "./+page.svelte";

type ProfilePageData = ComponentProps<typeof ProfilePage>["data"];

const profile = makeUser({
	id: 1,
	username: "zak",
	email: "zak@example.com",
	name: "Zak Story",
	profile_photo_url:
		"https://images.unsplash.com/photo-1500648767791-00dcc994a43e?auto=format&fit=crop&w=256&q=80",
});

const meta = {
	title: "Routes/Profile",
	component: RouteStoryHarness,
} satisfies Meta;

export default meta;
type Story = StoryObj;

function createData(overrides: Partial<ProfilePageData> = {}): ProfilePageData {
	return {
		...makeRouteLayoutData({ isAuthenticated: true }),
		isAuthenticated: true,
		profile,
		savedPaymentMethods: [],
		savedAddresses: [],
		errorMessage: "",
		...overrides,
	};
}

function createProfileApi(data: ProfilePageData) {
	return createApiStub({
		getProfile: async () => data.profile!,
		listSavedPaymentMethods: async () => data.savedPaymentMethods,
		listSavedAddresses: async () => data.savedAddresses,
	});
}

export const SignedOut: Story = {
	render: () =>
		renderRouteStory({
			component: ProfilePage,
			componentProps: {
				data: createData({
					isAuthenticated: false,
					profile: null,
				}),
			},
			api: createApiStub({
				getProfile: async () => {
					throw { status: 401 };
				},
				listSavedPaymentMethods: async () => [],
				listSavedAddresses: async () => [],
			}),
		}),
};

export const LoadedEmpty: Story = {
	render: () => {
		const data = createData();
		return renderRouteStory({
			component: ProfilePage,
			componentProps: { data },
			user: profile,
			api: createProfileApi(data),
		});
	},
};

export const LoadedWithSavedData: Story = {
	render: () => {
		const data = createData({
			savedPaymentMethods: [makeSavedPaymentMethod()],
			savedAddresses: [makeSavedAddress()],
		});
		return renderRouteStory({
			component: ProfilePage,
			componentProps: { data },
			user: profile,
			api: createProfileApi(data),
		});
	},
};

export const LoadError: Story = {
	render: () =>
		renderRouteStory({
			component: ProfilePage,
			componentProps: {
				data: createData({
					profile: null,
					errorMessage: "Unable to load your profile. Please try again.",
				}),
			},
			user: profile,
			api: createApiStub({
				getProfile: async () => {
					throw new Error("profile load failed");
				},
				listSavedPaymentMethods: async () => [],
				listSavedAddresses: async () => [],
			}),
		}),
};
