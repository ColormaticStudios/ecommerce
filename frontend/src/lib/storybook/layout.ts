import { makeStorefrontSettings } from "$lib/storybook/factories";

export function makeRouteLayoutData(
	overrides: Partial<{
		isAuthenticated: boolean;
		storefront: ReturnType<typeof makeStorefrontSettings>;
		draftPreview: { active: boolean; expires_at?: string | null };
	}> = {}
) {
	return {
		isAuthenticated: false,
		storefront: makeStorefrontSettings(),
		draftPreview: { active: false, expires_at: null },
		...overrides,
	};
}

export function makeAdminLayoutData(
	overrides: Partial<{
		isAuthenticated: boolean;
		isAdmin: boolean;
		accessError: string;
		storefront: ReturnType<typeof makeStorefrontSettings>;
		draftPreview: { active: boolean; expires_at?: string | null };
	}> = {}
) {
	return {
		...makeRouteLayoutData({ isAuthenticated: true }),
		isAdmin: true,
		accessError: "",
		...overrides,
	};
}
