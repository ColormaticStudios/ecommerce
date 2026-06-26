import type { CmsGlobalRegionModel, CmsNavigationModel } from "$lib/cms";

export function makeRouteLayoutData(
	overrides: Partial<{
		isAuthenticated: boolean;
		draftPreview: { active: boolean; expires_at?: string | null };
		cmsNavigation: CmsNavigationModel | null;
		cmsGlobalRegions: Record<string, CmsGlobalRegionModel>;
	}> = {}
) {
	return {
		isAuthenticated: false,
		draftPreview: { active: false, expires_at: null },
		cmsNavigation: null,
		cmsGlobalRegions: {},
		...overrides,
	};
}

export function makeAdminLayoutData(
	overrides: Partial<{
		isAuthenticated: boolean;
		isAdmin: boolean;
		accessError: string;
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
