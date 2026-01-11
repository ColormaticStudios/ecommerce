import { type API } from "$lib/api";
import { getProfile, userStore } from "$lib/user";
import { type UserModel } from "$lib/models";

export interface AdminAccessResult {
	isAuthenticated: boolean;
	isAdmin: boolean;
	user: UserModel | null;
}

export async function checkAdminAccess(api: API): Promise<AdminAccessResult> {
	api.tokenFromCookie();
	const isAuthenticated = api.isAuthenticated();
	if (!isAuthenticated) {
		return { isAuthenticated, isAdmin: false, user: null };
	}

	const user = await getProfile(api);
	if (!user) {
		return { isAuthenticated, isAdmin: false, user: null };
	}

	userStore.setUser(user);
	return { isAuthenticated, isAdmin: user.role === "admin", user };
}
